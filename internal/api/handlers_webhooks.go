package api

import (
	"net/http"

	"promptsheon/internal/webhook"
)

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}
	eps := s.webhooks.ListEndpoints()
	writeJSON(w, http.StatusOK, eps)
	return nil
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "webhook dispatcher not configured"}
	}
	var req struct {
		URL    string              `json:"url"`
		Events []webhook.EventType `json:"events"`
		Secret string              `json:"secret,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "invalid request body"}
	}
	if req.URL == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "url is required"}
	}
	if len(req.Events) == 0 {
		return &HTTPError{Status: http.StatusBadRequest, Message: "at least one event is required"}
	}
	ep := &webhook.Endpoint{
		ID:     generateID(),
		URL:    req.URL,
		Secret: req.Secret,
		Events: req.Events,
		Active: true,
	}
	s.webhooks.Register(ep)
	writeJSON(w, http.StatusCreated, ep)
	return nil
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "webhook dispatcher not configured"}
	}
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "id is required"}
	}
	s.webhooks.Remove(id)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
	return nil
}
