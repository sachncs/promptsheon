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
	s.db = store.NewRepositories(db)
	s.settingsReplicaID = "test-replica"
	s.settingsNotif = settings.NewNotifier()
	t.Cleanup(func() { _ = db.Close() })
	return s
}

// seedSetting writes a CRDT record directly so a test can
// stage DB state before exercising the API.
func seedSetting(t *testing.T, s *Server, key, value string) {
	t.Helper()
	rec := settings.CRDTRecord{
		Key:           key,
		Value:         value,
		ReplicaID:     "seed",
		WriteTS:       time.Now().UnixNano(),
		VersionVector: map[string]uint64{"seed": 1},
	}
	if err := s.db.SetSystemConfig(t.Context(), rec); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestHandleListSettings(t *testing.T) {
	s := settingsTestServer(t)
	seedSetting(t, s, "otl.endpoint", `"http://from-db:4317"`)
	seedSetting(t, s, "otl.insecure", "false")

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
	if len(got.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(got.Items))
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
	rec, err := s.db.GetSystemConfig(t.Context(), "otl.endpoint")
	if err != nil {
		t.Fatalf("get after put: %v", err)
	}
	if rec.Value != "http://otel:4317" {
		t.Fatalf("value: got %q, want %q", rec.Value, "http://otel:4317")
	}
	if rec.Tombstone {
		t.Fatalf("expected live row, got tombstone")
	}
	if rec.VersionVector["test-replica"] != 1 {
		t.Fatalf("expected test-replica=1, got %d", rec.VersionVector["test-replica"])
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

func TestHandleGetSetting_EnvOnly(t *testing.T) {
	s := settingsTestServer(t)
	t.Setenv("runtime.endpoint", "from-env")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/runtime.endpoint", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`"value":"from-env"`)) {
		t.Fatalf("GET env-only: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetSetting_EnvOverridesDB(t *testing.T) {
	s := settingsTestServer(t)
	seedSetting(t, s, "runtime.endpoint", "from-db")
	t.Setenv("runtime.endpoint", "from-env")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/runtime.endpoint", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`"value":"from-env"`)) {
		t.Fatalf("GET precedence: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetSetting_TombstoneReturns404(t *testing.T) {
	s := settingsTestServer(t)
	rec := settings.CRDTRecord{
		Key: "otl.endpoint", Value: "", ReplicaID: "tester",
		WriteTS: time.Now().UnixNano(), VersionVector: map[string]uint64{"tester": 1},
		Tombstone: true,
	}
	if err := s.db.SetSystemConfig(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/otl.endpoint", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("tombstoned GET: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteSetting(t *testing.T) {
	s := settingsTestServer(t)
	seedSetting(t, s, "otl.endpoint", `"http://x"`)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/otl.endpoint", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE: code=%d", rr.Code)
	}
	// The row is now a tombstone, not deleted.
	rec, err := s.db.GetSystemConfig(t.Context(), "otl.endpoint")
	if err != nil {
		t.Fatalf("post-delete get: %v", err)
	}
	if !rec.Tombstone {
		t.Fatalf("expected tombstone, got %+v", rec)
	}
}

func TestHandleDeleteSetting_MissingKey(t *testing.T) {
	s := settingsTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/never-existed", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	// Delete on a missing key now writes a tombstone; the
	// response is still 204.
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE missing: code=%d", rr.Code)
	}
}

func TestSettingsResponse_MasksSecret(t *testing.T) {
	got := settingsResponse("webhook.signing_secret", "plaintext", "tester", time.Time{})
	if got["value"] != "***" {
		t.Fatalf("secret not masked: got %v", got["value"])
	}
}

// TestHandleSetSetting_NotifierErrorPropagates verifies that
// a hot-reload subscriber failure surfaces to the API caller
// as a 500 (per the CRDT-aware notifier contract). The
// propagation path is settings.Resolver.Set → Notifier.Publish
// → HTTP 500.
func TestHandleSetSetting_NotifierErrorPropagates(t *testing.T) {
	s := settingsTestServer(t)
	s.settingsMode = "mutable"
	s.settingsNotif.Subscribe("otl.endpoint", func(_ string) error {
		return errSettingsReload
	})
	body, _ := json.Marshal(map[string]string{"value": "http://otel:4317"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/otl.endpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("PUT with failing subscriber: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

// errSettingsReload is the typed error used by the
// notifier-failure test. A sentinel kept local to the
// handlers_settings_test file so unrelated tests don't see
// it.
var errSettingsReload = settingsReloadError("reload failed")

type settingsReloadError string

func (e settingsReloadError) Error() string { return string(e) }
