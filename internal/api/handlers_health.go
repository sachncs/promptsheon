package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/sachncs/promptsheon/internal/buildinfo"
)

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	info := buildinfo.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		keyStatus: "healthy",
		"version": info.Version,
		"uptime":  time.Since(startTime).String(),
	})
	return nil
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) error {
	ready := map[string]any{
		keyStatus: "ready",
		"go":     runtime.Version(),
	}
	if s.db != nil {
		if err := s.db.Ping(r.Context()); err != nil {
			ready[keyStatus] = "not_ready"
			ready["database"] = "unreachable"
			writeJSON(w, http.StatusServiceUnavailable, ready)
			return nil
		}
		ready["database"] = "ok"
	}
	writeJSON(w, http.StatusOK, ready)
	return nil
}

// handleVersion returns the structured build info. The endpoint
// is intentionally unauthenticated so external uptime probes and
// load balancers can read the running version without an API key.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, buildinfo.Get())
	return nil
}
