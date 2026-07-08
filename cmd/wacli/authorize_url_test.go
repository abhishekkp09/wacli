package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthorizeURLCommandHuman(t *testing.T) {
	out := captureRootStdout(t, func() {
		if err := execute([]string{"authorize-url"}); err != nil {
			t.Fatalf("execute authorize-url: %v", err)
		}
	})

	if !strings.Contains(out, authorizeURL) {
		t.Fatalf("authorize-url output = %q, want to contain %q", out, authorizeURL)
	}
	if !strings.Contains(out, authorizeMessage) {
		t.Fatalf("authorize-url output = %q, want to contain %q", out, authorizeMessage)
	}
}

func TestAuthorizeURLCommandJSON(t *testing.T) {
	out := captureRootStdout(t, func() {
		if err := execute([]string{"--json", "authorize-url"}); err != nil {
			t.Fatalf("execute authorize-url --json: %v", err)
		}
	})

	var got struct {
		Success bool `json:"success"`
		Data    struct {
			ConnectURL string `json:"connect_url"`
			Message    string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("authorize-url JSON = %q: %v", out, err)
	}
	if !got.Success || got.Data.ConnectURL != authorizeURL || got.Data.Message != authorizeMessage {
		t.Fatalf("authorize-url JSON = %+v, want connect_url %q message %q", got, authorizeURL, authorizeMessage)
	}
}
