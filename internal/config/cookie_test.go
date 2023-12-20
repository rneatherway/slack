//go:build network
// +build network

// These tests require that you be logged into Slack on the current machine.
// You must also pass '-tags network' to 'go test'
package config

import "testing"

func TestGetCookie(t *testing.T) {
	cookie, err := GetCookie()
	if err != nil {
		t.Error(err)
	}

	if cookie == "" {
		t.Error("empty cookie")
	}
}
