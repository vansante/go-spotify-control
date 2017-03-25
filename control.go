package spotifycontrol

import (
	"net/http"
	"net"
	"time"
	"fmt"
	"sync"
	"encoding/json"
	"io/ioutil"
	"errors"
	"bytes"
	"strconv"
	"strings"
)

const (
	START_PORT = 4370
	END_PORT   = 4400
)

const SPOTIFY_OAUTH_TOKEN_URL = "https://open.spotify.com/token"

const (
	URL_CSRF_TOKEN = "/simplecsrf/token.json"
	URL_PAUSE      = "/remote/pause.json"
	URL_STATUS     = "/remote/status.json"
)

type SpotifyControl struct {
	client *http.Client
	host   string
	port   int
	csrf   string
	oauth  string
}

type StatusTrackResource struct {
	Name string
	Uri  string
}

type StatusTrack struct {
	Length    int
	TrackType string

	Track  StatusTrackResource `json:"track_resource"`
	Artist StatusTrackResource `json:"artist_resource"`
	Album  StatusTrackResource `json:"album_resource"`
}

type Status struct {
	Version       int
	ClientVersion string
	Playing       bool
	Shuffle       bool
	Repeat        bool
	PlayEnabled   bool
	PrevEnabled   bool
	NextEnabled   bool
	Track         StatusTrack
}

func NewSpotifyControl(host string, timeout time.Duration) (cntrl *SpotifyControl, err error) {
	if host == "" {
		host = "127.0.0.1"
	}
	if timeout == 0 {
		timeout = 500 * time.Millisecond
	}
	// Use a http client with low timeout.
	// https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779#.5dz971e7g
	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: timeout,
		}).Dial,
		TLSHandshakeTimeout: timeout,
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: netTransport,
	}

	cntrl = &SpotifyControl{
		client: client,
		host:   host,
	}
	cntrl.port, err = cntrl.findPort()
	if err != nil {
		return
	}
	cntrl.oauth, err = cntrl.getOauthToken()
	if err != nil {
		return
	}
	cntrl.csrf, err = cntrl.getCsrfToken()
	return
}

func (cntrl *SpotifyControl) Pause() (status *Status, err error) {
	return cntrl.SetPauseState(true)
}

func (cntrl *SpotifyControl) UnPause() (status *Status, err error) {
	return cntrl.SetPauseState(false)
}

func (cntrl *SpotifyControl) SetPauseState(paused bool) (status *Status, err error) {
	jsonMap, bodyBuf, err := cntrl.doJsonRequest("GET", fmt.Sprintf("%s?csrf=%s&oauth=%s&pause=%v", URL_PAUSE, cntrl.csrf, cntrl.oauth, paused))
	if err != nil {
		return
	}
	_, _, err = cntrl.getErrorFromJSON(jsonMap)
	if err != nil {
		return
	}
	status = &Status{}
	err = json.Unmarshal(bodyBuf, status)
	return
}

func (cntrl *SpotifyControl) getOauthToken() (token string, err error) {
	req, err := http.NewRequest("GET", SPOTIFY_OAUTH_TOKEN_URL, strings.NewReader(""))
	if err != nil {
		return
	}
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com")
	req.Header.Set("User-Agent", "GO Spotify Control")
	resp, err := cntrl.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bodyBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	jsonMap, err := cntrl.parseJSON(bodyBuf)
	token, ok := jsonMap["t"].(string)
	if !ok || token == "" {
		err = errors.New("OAuth token not found or invalid in Spotify API response")
	}
	return
}

func (cntrl *SpotifyControl) getCsrfToken() (token string, err error) {
	jsonMap, _, err := cntrl.doJsonRequest("GET", URL_CSRF_TOKEN)
	if err != nil {
		return
	}
	_, _, err = cntrl.getErrorFromJSON(jsonMap)
	if err != nil {
		return
	}

	token, ok := jsonMap["token"].(string)
	if !ok || token == "" {
		err = errors.New("CSRF token key not found or invalid in Spotify API response")
	}
	return
}

func (cntrl *SpotifyControl) findPort() (port int, err error) {
	wg := sync.WaitGroup{}
	wg.Add(END_PORT - START_PORT + 1)

	for currentPort := START_PORT; currentPort <= END_PORT; currentPort++ {
		go func(tryPort int) {
			url := fmt.Sprintf("http://%s:%d%s", cntrl.host, tryPort, URL_STATUS)
			resp, err := cntrl.client.Get(url)
			if err == nil {
				resp.Body.Close()
				port = tryPort
			}
			//log.Printf("Trying currentPort %d: %v | %v", tryPort, err, resp)
			wg.Done()
		}(currentPort)
	}
	wg.Wait()

	if port == 0 {
		err = fmt.Errorf("Spotify port in range %d-%d not found, is it running?", START_PORT, END_PORT)
	}
	return
}

func (cntrl *SpotifyControl) getErrorFromJSON(jsonMap map[string]interface{}) (code int, message string, err error) {
	errData, ok := jsonMap["error"].(map[string]interface{})
	if !ok {
		return
	}
	codeStr, ok := errData["type"].(string)
	if ok {
		code64, _ := strconv.ParseInt(codeStr, 10, 32)
		code = int(code64)
	}
	message, _ = errData["message"].(string)
	err = fmt.Errorf("Spotify API error %d: %s", code, message)
	return
}

func (cntrl *SpotifyControl) doJsonRequest(method, url string) (jsonMap map[string]interface{}, bodyBuf []byte, err error) {
	resp, err := cntrl.doRequest(method, url, []byte{})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bodyBuf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	jsonMap, err = cntrl.parseJSON(bodyBuf)
	return
}

func (cntrl *SpotifyControl) parseJSON(jsonBuf []byte) (jsonMap map[string]interface{}, err error) {
	var jsonData interface{}
	err = json.Unmarshal(jsonBuf, &jsonData)
	if err != nil {
		return
	}

	jsonMap, ok := jsonData.(map[string]interface{})
	if !ok {
		err = errors.New("Uknown JSON object could not be parsed")
		return
	}
	return
}

func (cntrl *SpotifyControl) doRequest(method, url string, body []byte) (resp *http.Response, err error) {
	url = fmt.Sprintf("http://%s:%d%s", cntrl.host, cntrl.port, url)
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com")
	req.Header.Set("User-Agent", "GO Spotify Control")
	resp, err = cntrl.client.Do(req)
	return
}
