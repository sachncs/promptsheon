//go:build tests_migration


package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/alerting"
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestHandleListAlertRules(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlertRules_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/rules", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAlertRule(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	body := mustMarshal(t, map[string]any{"name": "test-rule", "type": "threshold", "severity": "high", "threshold": 10.0})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-rule" {
		t.Errorf("expected test-rule, got %v", resp["name"])
	}
}

func TestHandleCreateAlertRule_NilManager(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{"name": "test-rule", "type": "threshold"})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreateAlertRule_EmptySeverityRejected locks in DEF-18 fix (c0.5).
// Before the fix an empty severity string was accepted and persisted as
// the zero value; downstream consumers received an empty label.
func TestHandleCreateAlertRule_EmptySeverityRejected(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	cases := []struct {
		name     string
		severity string
		wantCode int
	}{
		{"empty severity rejected", "", http.StatusBadRequest},
		{"unknown severity rejected", "nuclear", http.StatusBadRequest},
		{"low accepted", "low", http.StatusCreated},
		{"medium accepted", "medium", http.StatusCreated},
		{"high accepted", "high", http.StatusCreated},
		{"critical accepted", "critical", http.StatusCreated},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := mustMarshal(t, map[string]any{"name": "rule-" + c.name, "type": "threshold", "severity": c.severity, "threshold": 1.0})
			req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			s.ServeHTTP(rr, req)
			if rr.Code != c.wantCode {
				t.Errorf("severity=%q: status=%d, want %d (body: %s)", c.severity, rr.Code, c.wantCode, rr.Body.String())
			}
		})
	}
}

func TestHandleCreateAlertRule_MissingName(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	body := mustMarshal(t, map[string]any{"type": "threshold"})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)
	rule := &alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh}
	am.AddRule(rule)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test" {
		t.Errorf("expected test, got %v", resp["name"])
	}
}

func TestHandleGetAlertRule_NotFound(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAlertRule_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)
	rule := &alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh}
	am.AddRule(rule)
	s := newTestServer(t)
	s.alertingManager = am

	body := mustMarshal(t, map[string]any{"name": "updated-rule", "enabled": false, "threshold": 20.0})
	req := httptest.NewRequest("PUT", "/api/v1/alerts/rules/rule1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "updated-rule" {
		t.Errorf("expected updated-rule, got %v", resp["name"])
	}
	if resp["enabled"] != false {
		t.Errorf("expected enabled false, got %v", resp["enabled"])
	}
}

func TestHandleDeleteAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)
	am.AddRule(&alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh})
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("DELETE", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlerts(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("GET", "/api/v1/alerts/active", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlerts_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/active", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleResolveAlert_NotFound(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("PUT", "/api/v1/alerts/active/nonexistent/resolve", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleResolveAlert_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("PUT", "/api/v1/alerts/active/nonexistent/resolve", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAddNotificationGroup(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil)

	body := mustMarshal(t, map[string]any{"name": "slack-group", "channels": []string{"slack"}})
	req := httptest.NewRequest("POST", "/api/v1/alerts/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "slack-group" {
		t.Errorf("expected slack-group, got %v", resp["name"])
	}
}

func TestHandleAddNotificationGroup_NilManager(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{"name": "group", "channels": []string{"email"}})
	req := httptest.NewRequest("POST", "/api/v1/alerts/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Webhook Handler Tests
// ---------------------------------------------------------------------------
