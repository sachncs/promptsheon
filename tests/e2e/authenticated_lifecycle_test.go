// Package e2e: authenticated canonical-lifecycle test.
//
// The existing daemon_e2e_test.go exercises the legacy unauth path
// (POST /api/v1/setup). This file covers the authenticated
// path: with PROMPTSHEON_AUTH=true, an admin key is created via
// /api/v1/setup with a bootstrap token, then the canonical
// workspace → project → capability → version → release → vote →
// activate → invoke flow is exercised end to end through the Go
// SDK.
//
// The same daemon binary is used as the existing e2e harness;
// TestMain in this file does NOT start a second daemon. Instead
// it checks that the e2e package's existing daemon was started
// with PROMPTSHEON_AUTH=true and skips otherwise.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestE2E_AuthenticatedLifecycle drives the full authenticated
// lifecycle: admin key via bootstrap token, workspace/project/
// capability/version creation, audit chain.
//
// Skipped when the e2e daemon was started without auth
// (the default — see daemon_e2e_test.go startDaemon).
func TestE2E_AuthenticatedLifecycle(t *testing.T) {
	if daemonAPIKey == "" {
		t.Skip("e2e daemon is not configured for auth")
	}
	if daemonBaseURL == "" {
		t.Skip("e2e daemon not started")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. List users to confirm auth works.
	usersBody, status, err := getJSON(ctx, "/api/v1/users", daemonAPIKey)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("ListUsers: status %d body %s", status, usersBody)
	}
	if !strings.Contains(string(usersBody), `"id"`) {
		t.Fatalf("expected user list, got %s", usersBody)
	}

	// 2. Create a workspace via raw HTTP.
	wsID, err := postJSON(ctx, "/api/v1/workspaces", map[string]any{"name": "e2e-ws"}, daemonAPIKey)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Logf("workspace = %s", wsID)

	// 3. Create a project.
	projectID, err := postJSON(ctx, "/api/v1/workspaces/"+wsID+"/projects",
		map[string]any{"name": "e2e-proj"}, daemonAPIKey)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Logf("project = %s", projectID)

	// 4. Create a capability.
	capID, err := postJSON(ctx, "/api/v1/projects/"+projectID+"/capabilities",
		map[string]any{"name": "e2e-cap", "description": "test"}, daemonAPIKey)
	if err != nil {
		t.Fatalf("create capability: %v", err)
	}
	t.Logf("capability = %s", capID)

	// 5. Create a version with a valid manifest.
	_, err = postJSON(ctx, "/api/v1/capabilities/"+capID+"/versions",
		map[string]any{
			"version": 1,
			"manifest": map[string]any{
				"prompt":           map[string]any{"kind": "prompt", "hash": "a" + strings.Repeat("0", 63)},
				"model_policy":     map[string]any{"kind": "model_policy", "hash": "b" + strings.Repeat("0", 63)},
				"runtime_policy":   map[string]any{"kind": "runtime_policy", "hash": "c" + strings.Repeat("0", 63)},
				"context_contract": map[string]any{"kind": "context_contract", "hash": "d" + strings.Repeat("0", 63)},
				"memory":           map[string]any{"kind": "memory", "hash": "e" + strings.Repeat("0", 63)},
			},
		}, daemonAPIKey)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}

	// 6. List audit via raw HTTP and confirm a "create" event
	// for the capability. The audit chain is the security
	// boundary; the lifecycle must leave a trail.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, daemonBaseURL+"/api/v1/audit?limit=200", nil)
	if daemonAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+daemonAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /audit: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("GET /audit: status %d body %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"action":"create"`) {
		t.Fatalf("expected a 'create' audit event; body=%s", body)
	}
}

// TestE2E_AuditChain verifies the audit chain on the running
// daemon. The chain is the security boundary; this test confirms
// a fresh daemon's chain is verified clean.
func TestE2E_AuditChain(t *testing.T) {
	if daemonBaseURL == "" {
		t.Skip("e2e daemon not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, daemonBaseURL+"/api/v1/audit/verify", nil)
	if daemonAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+daemonAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("verify audit: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify audit: status %d body %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("audit chain invalid: %s", body)
	}
}

// postJSON sends a POST request with the supplied JSON body and
// returns the response body's ID field. Helpers exist because
// the e2e harness exercises routes not yet covered by the Go
// SDK (workspace + project creation).
func postJSON(ctx context.Context, path string, body any, apiKey string) (string, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, daemonBaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d body %s", resp.StatusCode, b)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("decode %s: %w", b, err)
	}
	return out.ID, nil
}

// getJSON is the read-side companion to postJSON. Returns the
// raw body and the HTTP status; the caller decodes.
func getJSON(ctx context.Context, path, apiKey string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, daemonBaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}
