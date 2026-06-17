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
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"go":     runtime.Version(),
	})
	return nil
}
