package api

import (
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "healthy",
		"version": "0.1.0",
		"uptime":  time.Since(startTime).String(),
	})
	return nil
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) error {
	ready := map[string]any{
		"status": "ready",
		"go":     runtime.Version(),
	}
	if s.db != nil {
		if err := s.db.Ping(r.Context()); err != nil {
			ready["status"] = "not_ready"
			ready["database"] = "unreachable"
			writeJSON(w, http.StatusServiceUnavailable, ready)
			return nil
		}
		ready["database"] = "ok"
	}
	writeJSON(w, http.StatusOK, ready)
	return nil
}
