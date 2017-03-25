package spotifycontrol

import (
	"testing"
	"time"
)

func TestSpotifyControl(t *testing.T) {
	cntrl, err := NewSpotifyControl("", 0)
	if err != nil || cntrl.port <= 0 {
		t.Fail()
		t.Logf("Port finding failed: %v", err)
	} else {
		t.Logf("Port found: %d", cntrl.port)
	}

	resp, err := cntrl.doRequest("GET", URL_CSRF_TOKEN, nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fail()
		t.Logf("CSRF request failed: %v", err)
	}

	token, err := cntrl.getCsrfToken()
	if err != nil || token == "" {
		t.Fail()
		t.Logf("Get CSRF token failed: %v", err)
	} else {
		t.Logf("Get CSRF token: %s", token)
	}

	token, err = cntrl.getOauthToken()
	if err != nil || token == "" {
		t.Fail()
		t.Logf("Get oauth token failed: %v", err)
	} else {
		t.Logf("Get oauth token: %s", token)
	}

	status, err := cntrl.Pause()
	if err != nil || status.Playing {
		t.Fail()
		t.Logf("Pause failed: %v", err)
	} else {
		t.Log("Pause succeeded")
	}

	time.Sleep(1 * time.Second)

	status, err = cntrl.UnPause()
	if err != nil || !status.Playing {
		t.Fail()
		t.Logf("Unpause failed: %v", err)
	} else {
		t.Log("Unpause succeeded")
	}
}