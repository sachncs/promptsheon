package main

// http.go: small HTTP helpers used by the workspace / project /
// capability / release / provider subcommands. They intentionally
// stay inline rather than depending on net/http/httptest or the
// SDK so the CLI stays a thin wrapper around the running daemon.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const cliKeyName = "name"

func serverURL() string {
	u := os.Getenv("PROMPTSHEON_SERVER")
	if u == "" {
		u = "http://localhost:8080"
	}
	return strings.TrimRight(u, "/")
}

// validateLocalURL restricts HTTP requests to loopback addresses
// to prevent SSRF. The CLI only talks to a local daemon.
func validateLocalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := parsed.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("remote requests not allowed: %s", host)
	}
	return nil
}

func httpGet(rawURL string, v any) error {
	if err := validateLocalURL(rawURL); err != nil {
		return err
	}
	// #nosec G704,G107 -- URL validated to localhost by validateLocalURL above.
	// The CLI only connects to the local daemon; SSRF is not a concern.
	resp, err := http.Get(rawURL)
	if err != nil {
		return fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return fmt.Errorf("GET %s: %s", rawURL, resp.Status)
		}
		return fmt.Errorf("GET %s: %s (%s)", rawURL, resp.Status, body["error"])
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func httpPost(rawURL string, body, v any) error {
	if err := validateLocalURL(rawURL); err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	// #nosec G704,G107 -- URL validated to localhost by validateLocalURL above.
	resp, err := http.Post(rawURL, "application/json", strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("POST %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		var errBody map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err != nil {
			return fmt.Errorf("POST %s: %s", rawURL, resp.Status)
		}
		return fmt.Errorf("POST %s: %s (%s)", rawURL, resp.Status, errBody["error"])
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func httpDelete(rawURL string) error {
	if err := validateLocalURL(rawURL); err != nil {
		return err
	}
	// #nosec G704 -- URL validated to localhost by validateLocalURL above.
	req, err := http.NewRequest("DELETE", rawURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", rawURL, err)
	}
	// #nosec G704 -- URL validated to localhost by validateLocalURL above.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", rawURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("DELETE %s: %s", rawURL, resp.Status)
	}
	return nil
}
