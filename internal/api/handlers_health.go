package api

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/sachncs/promptsheon/internal/buildinfo"
)

const dbStatusOK = "ok"

var startTime = time.Now()

// handleHealth is the liveness probe. It returns 200 whenever
// the HTTP server is processing requests; per the K8s
// liveness/readiness contract, liveness is "is the process
// alive" not "is it healthy". Deeper checks live in /ready.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) error {
	info := buildinfo.Get()
	body := map[string]any{
		auditKeyStatus:  "healthy",
		auditKeyVersion: info.Version,
		"uptime":        time.Since(startTime).String(),
	}
	// PERF-MEM-1: include runtime.MemStats on /health when the
	// caller asks for it via ?debug=1. Production probes do not
	// include the query string so the MemStats read is skipped.
	if r.URL.Query().Get("debug") == "1" {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		body["memory"] = map[string]any{
			"alloc_bytes":       ms.Alloc,
			"sys_bytes":         ms.Sys,
			"heap_objects":      ms.HeapObjects,
			"num_goroutine":     runtime.NumGoroutine(),
			"num_gc":            ms.NumGC,
			"gc_pause_total_ns": ms.PauseTotalNs,
		}
	}
	writeJSON(w, http.StatusOK, body)
	// Best-effort: flush any buffered spans so a health probe
	// that happens to double as a trace-flush gets the data
	// out the door. A failure here is not fatal — /health
	// always returns 200 because the daemon IS up.
	if s.spans != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()
		_ = s.spans.Flush(ctx)
	}
	return nil
}

// handleReady returns 200 only when every downstream the daemon
// depends on for correct operation is reachable:
//
//   - The SQLite database responds to Ping (operator's data).
//   - The OTel tracer is reachable (the daemon otherwise can't
//     ship traces to the operator's collector; failures here
//     will not stall the request path but they do mean the
//     observability story is broken).
//   - The audit worker queue has room (degraded audit means
//     we may be losing entries; the queue depth is exposed
//     so K8s readiness can shed traffic before drops begin).
//
// A failed probe returns 503 with a body naming the failing
// component so operators can triage from the kubelet log
// without digging through the daemon log.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) error {
	ready := map[string]any{
		auditKeyStatus: "ready",
		"go":           runtime.Version(),
	}
	if err := s.db.Ping(r.Context()); err != nil {
		ready[auditKeyStatus] = "not_ready"
		ready["database"] = "unreachable"
		ready["reason"] = "database ping failed: " + err.Error()
		writeJSON(w, http.StatusServiceUnavailable, ready)
		return nil
	}
	ready["database"] = dbStatusOK
	// Leader election: report the current holder so operators
	// can tell which replica is the writer.
	if s.elector != nil {
		leader, _ := s.elector.Current(r.Context())
		ready["leader"] = leader.Identity
		ready["leader_expires_at"] = leader.ExpiresAt.UTC().Format(time.RFC3339)
		if leader.IsLeader {
			ready["role"] = "leader"
		} else {
			ready["role"] = "follower"
		}
	}
	// Audit worker queue depth. A near-full queue means drops
	// are imminent; surface it so the readiness probe can
	// shed traffic before the operator sees a 5xx-class event.
	if cap(s.auditQueue) > 0 {
		depth := len(s.auditQueue)
		ready["audit_queue_depth"] = depth
		ready["audit_queue_capacity"] = cap(s.auditQueue)
		if depth*4 > cap(s.auditQueue)*3 { // 75% threshold
			ready[auditKeyStatus] = "degraded"
			ready["reason"] = "audit worker queue near full"
		}
	}
	// OTel tracer reachability. Flush drains any buffered spans
	// to the export pipeline; a successful Flush means the
	// collector is accepting spans. We allow 200ms — Flush
	// blocks on the local queue, not the network, so this is
	// fast in practice.
	if s.spans != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()
		if err := s.spans.Flush(ctx); err != nil {
			ready[auditKeyStatus] = "degraded"
			ready["otel_exporter"] = "flush failed: " + err.Error()
		} else {
			ready["otel_exporter"] = dbStatusOK
		}
	}
	if ready[auditKeyStatus] == "ready" {
		writeJSON(w, http.StatusOK, ready)
	} else {
		writeJSON(w, http.StatusServiceUnavailable, ready)
	}
	return nil
}

// handleVersion returns the structured build info. The endpoint
// is intentionally unauthenticated so external uptime probes and
// load balancers can read the running version without an API key.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, buildinfo.Get())
	return nil
}
