package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
)

// settingsTestServer builds a Server backed by a real
// *store.SQLite (the mockRepo is in-memory and doesn't persist
// across calls). The settings layer's CRUD contract is
// exercised against a real DB so the test catches real
// migration drift.
func settingsTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := t.TempDir() + "/settings.db"
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	s := newTestServer(t)
	s.db = db
	t.Cleanup(func() { _ = db.Close() })
	return s
}

func TestHandleListSettings(t *testing.T) {
	s := settingsTestServer(t)
	ctx := t.Context()
	_ = s.db.SetSystemConfig(ctx, "otl.endpoint", `"http://from-db:4317"`, "tester")
	_ = s.db.SetSystemConfig(ctx, "otl.insecure", "false", "tester")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings: code=%d body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.Items))
	}
}

func TestHandleSetSetting_MutableMode(t *testing.T) {
	s := settingsTestServer(t)
	s.settingsMode = "mutable"

	body, _ := json.Marshal(map[string]string{"value": "http://otel:4317"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/otl.endpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT: code=%d body=%s", rr.Code, rr.Body.String())
	}
	v, _, err := s.db.GetSystemConfig(t.Context(), "otl.endpoint")
	if err != nil {
		t.Fatalf("get after put: %v", err)
	}
	if v != "http://otel:4317" {
		t.Fatalf("value: got %q, want %q", v, "http://otel:4317")
	}
}

func TestHandleSetSetting_EnvOnlyMode(t *testing.T) {
	s := settingsTestServer(t)
	s.settingsMode = "env-only"

	body, _ := json.Marshal(map[string]string{"value": "http://otel:4317"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/otl.endpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("PUT in env-only mode: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetSetting_RejectsSecret(t *testing.T) {
	s := settingsTestServer(t)
	settings.RegisterSecretKey("webhook.signing_secret")

	body, _ := json.Marshal(map[string]string{"value": "leak-me"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/webhook.signing_secret", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT on secret key: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetSetting_MissingKey(t *testing.T) {
	s := settingsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/does.not.exist", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET missing: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteSetting(t *testing.T) {
	s := settingsTestServer(t)
	_ = s.db.SetSystemConfig(t.Context(), "otl.endpoint", `"http://x"`, "tester")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/otl.endpoint", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE: code=%d", rr.Code)
	}
}

func TestSettingsResponse_MasksSecret(t *testing.T) {
	settings.RegisterSecretKey("webhook.signing_secret")
	got := settingsResponse("webhook.signing_secret", "plaintext", "tester", time.Time{})
	if got["value"] != "***" {
		t.Fatalf("secret not masked: got %v", got["value"])
	}
}
