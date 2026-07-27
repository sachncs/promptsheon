package backend

import (
	"log/slog"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/backend/metrics"
)

func TestHandleMetricsSummary(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleMetricsSummary_AuditDropped locks in OBS-AUDIT-1: the
// /api/v1/metrics/summary JSON response surfaces auditDropped
// under pipeline_metrics.audit_dropped.

func TestHandleMetricsSummary_AuditDropped(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()
	s.collector.SetAuditDropped(42)

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	pm, ok := resp["pipeline_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("missing pipeline_metrics in response: %v", resp)
	}
	if pm["audit_dropped"] != float64(42) {
		t.Errorf("expected audit_dropped=42, got %v", pm["audit_dropped"])
	}
}

// TestHandleMetricsSummary_AuditQueueLatency locks in OBS-AUDIT-2:
// the audit queue latency histogram is exposed in
// /api/v1/metrics/summary under pipeline_metrics.

func TestHandleMetricsSummary_AuditQueueLatency(t *testing.T) {
	s := newTestServer(t)
	c := metrics.NewCollector()
	s.collector = c
	c.ObserveAuditQueue(0.005)
	c.ObserveAuditQueue(0.020)
	c.ObserveAuditQueue(0.080)

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	pm, ok := resp["pipeline_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("missing pipeline_metrics in response: %v", resp)
	}
	if _, ok := pm["audit_queue_avg_secs"]; !ok {
		t.Errorf("missing audit_queue_avg_secs in pipeline_metrics: %v", pm)
	}
	if _, ok := pm["audit_queue_p95_secs"]; !ok {
		t.Errorf("missing audit_queue_p95_secs in pipeline_metrics: %v", pm)
	}
	if _, ok := pm["audit_queue_p99_secs"]; !ok {
		t.Errorf("missing audit_queue_p99_secs in pipeline_metrics: %v", pm)
	}
}


func TestHandleMetricsPrometheus(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()

	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleMetricsPrometheus_NotConfigured(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleLogsStream_WithHub(t *testing.T) {
	hub := NewHub(slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})))
	go hub.Run()

	s := newTestServer(t)
	s.logHub = hub

	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil)
	// Cancel context after a short delay so SSE handler exits
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
}


func TestHandleLogsStream_NotConfigured(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Provider Handler Tests
// ---------------------------------------------------------------------------
