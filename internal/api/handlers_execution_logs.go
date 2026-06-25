package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func (s *Server) handleListExecutionLogs(w http.ResponseWriter, r *http.Request) error {
	filter := models.ExecutionLogFilter{}

	if v := r.URL.Query().Get("prompt_id"); v != "" {
		filter.PromptID = v
	}
	if v := r.URL.Query().Get("provider"); v != "" {
		filter.Provider = v
	}
	if v := r.URL.Query().Get("model"); v != "" {
		filter.Model = v
	}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = v
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := r.URL.Query().Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = &t
		}
	}

	filter.Limit = 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			filter.Limit = n
		}
	}
	filter.Offset = 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	logs, err := s.db.ListExecutionLogs(r.Context(), filter)
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, logs)
	return nil
}

func (s *Server) handleGetExecutionLog(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}

	log, err := s.db.GetExecutionLog(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	writeJSON(w, http.StatusOK, log)
	return nil
}
