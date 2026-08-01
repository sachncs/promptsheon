package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/webhook"
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestHandleListWebhooks_NilDispatcher(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListWebhooks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{
		"url":    "https://example.com/webhook",
		"events": []string{"eval.completed"},
	})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["url"] != "https://example.com/webhook" {
		t.Errorf("expected url, got %v", resp["url"])
	}
}

func TestHandleCreateWebhook_NilDispatcher(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{
		"url":    "https://example.com/webhook",
		"events": []string{"eval.completed"},
	})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook_MissingURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{"events": []string{"eval.completed"}})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook_NoEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{"url": "https://example.com/hook"})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteWebhook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	req := httptest.NewRequest("DELETE", "/api/v1/webhooks/wh1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Webhook URL Validation Tests
// ---------------------------------------------------------------------------

func TestValidateWebhookURL_InvalidScheme(t *testing.T) {
	err := ValidateWebhookURL("ftp://example.com/hook")
	if err == nil {
		t.Error("expected error for ftp scheme")
	}
}

func TestValidateWebhookURL_HTTPRejected(t *testing.T) {
	err := ValidateWebhookURL("http://example.com/hook")
	if err == nil {
		t.Error("expected error for http scheme (only https is accepted)")
	}
}

func TestValidateWebhookURL_MissingHost(t *testing.T) {
	err := ValidateWebhookURL("http:///path")
	if err == nil {
		t.Error("expected error for missing host")
	}
}

func TestValidateWebhookURL_LocalhostBlocked(t *testing.T) {
	err := ValidateWebhookURL("https://localhost:8080/hook")
	if err == nil {
		t.Error("expected error for localhost")
	}
}

func TestValidateWebhookURL_MetadataHostname(t *testing.T) {
	err := ValidateWebhookURL("https://metadata.google.internal")
	if err == nil {
		t.Error("expected error for metadata hostname")
	}
}

// TestValidateWebhookURL_AWSMetadataIP locks in the SEC-4a
// acceptance literal: https://169.254.169.254/... must be
// rejected as a link-local address. The hostname form was
// already covered by TestValidateWebhookURL_MetadataHostname.

func TestValidateWebhookURL_AWSMetadataIP(t *testing.T) {
	err := ValidateWebhookURL("https://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Error("expected error for AWS IMDS link-local address")
	}
}

// ---------------------------------------------------------------------------
// UsageTracker Tests
// ---------------------------------------------------------------------------
