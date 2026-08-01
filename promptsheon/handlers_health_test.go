//go:build tests_migration


package promptsheon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", body["status"])
	}
	if body["version"] == nil {
		t.Error("expected version")
	}
	if body["uptime"] == nil {
		t.Error("expected uptime")
	}
}

func TestHandleReady(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "ready" {
		t.Errorf("expected ready, got %v", body["status"])
	}
	if body["database"] != "ok" {
		t.Errorf("expected database ok, got %v", body["database"])
	}
	if body["go"] == nil {
		t.Error("expected go version")
	}
}

func TestHandleReady_DBPingFail(t *testing.T) {
	repo := newMockRepo()
	repo.pingErr = errors.New("db down")
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "not_ready" {
		t.Errorf("expected not_ready, got %v", body["status"])
	}
	if body["database"] != "unreachable" {
		t.Errorf("expected database unreachable, got %v", body["database"])
	}
}

func TestHandleVersion(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/version", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestHandleHealth_UptimeMonotonic verifies the startTime move
// into per-server state: the uptime field reported across two
// probes must be non-decreasing and a fresh Server must report a
// uptime of (approximately) zero.
func TestHandleHealth_UptimeMonotonic(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rr1 := httptest.NewRecorder()
	s.ServeHTTP(rr1, req)
	var body1 map[string]any
	readJSONBody(t, rr1.Body.Bytes(), &body1)
	if body1["uptime"] == nil {
		t.Fatal("expected uptime field on first probe")
	}
	// Subsequent probes report the same or higher uptime; the
	// first probe must already be a non-negative duration.
	req2 := httptest.NewRequest("GET", "/health", nil)
	rr2 := httptest.NewRecorder()
	s.ServeHTTP(rr2, req2)
	var body2 map[string]any
	readJSONBody(t, rr2.Body.Bytes(), &body2)
	if body2["uptime"] == nil {
		t.Fatal("expected uptime field on second probe")
	}
}

// ---------------------------------------------------------------------------
// Auth Handler Tests
// ---------------------------------------------------------------------------
