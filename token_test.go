//go:build network
// +build network

// These tests require that you be logged into Slack on the current machine.
// You must also pass '-tags network' to 'go test'
package slack

import "testing"

func TestGetCookieAuth(t *testing.T) {
	// Replace <team> with the name of a team that you are logged into on this machine.
	auth, err := GetCookieAuth("<team>")
	if err != nil {
		t.Error(err)
	}

	if auth.Token == "" {
		t.Fatal("empty token")
	}

	if auth.Cookies["d"] == "" {
		t.Fatal("empty cookie")
	}
}
