package api

import (
	"net/http"
	"time"

	"promptsheon/internal/alerting"
)

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		writeJSON(w, http.StatusOK, []*alerting.AlertRule{})
		return nil
	}
	rules := s.alertingManager.ListRules()
	if rules == nil {
		rules = []*alerting.AlertRule{}
	}
	writeJSON(w, http.StatusOK, rules)
	return nil
}

func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}

	var req struct {
		Name      string            `json:"name"`
		Type      string            `json:"type"`
		Severity  string            `json:"severity"`
		Threshold float64           `json:"threshold"`
		Duration  int               `json:"duration_minutes"`
		Window    int               `json:"window_minutes"`
		Config    map[string]any    `json:"config,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name == "" || req.Type == "" {
		return badRequest("name and type are required")
	}

	now := time.Now()
	rule := &alerting.AlertRule{
		ID:        generateID(),
		Name:      req.Name,
		Type:      req.Type,
		Severity:  alerting.Severity(req.Severity),
		Enabled:   true,
		Threshold: req.Threshold,
		Duration:  req.Duration,
		Window:    req.Window,
		Config:    req.Config,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.alertingManager.AddRule(rule)
	s.audit(r.Context(), "create", "alert_rule:"+rule.ID, map[string]any{"name": rule.Name})
	writeJSON(w, http.StatusCreated, rule)
	return nil
}

func (s *Server) handleGetAlertRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}
	rule, ok := s.alertingManager.GetRule(id)
	if !ok {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, rule)
	return nil
}

func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}

	existing, ok := s.alertingManager.GetRule(id)
	if !ok {
		return ErrNotFound
	}

	var req struct {
		Name      *string           `json:"name"`
		Enabled   *bool             `json:"enabled"`
		Threshold *float64          `json:"threshold"`
		Config    map[string]any    `json:"config,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Threshold != nil {
		existing.Threshold = *req.Threshold
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	existing.UpdatedAt = time.Now()

	s.alertingManager.AddRule(existing)
	s.audit(r.Context(), "update", "alert_rule:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}
	s.alertingManager.RemoveRule(id)
	s.audit(r.Context(), "delete", "alert_rule:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		writeJSON(w, http.StatusOK, []*alerting.Alert{})
		return nil
	}
	alerts := s.alertingManager.ListAlerts()
	if alerts == nil {
		alerts = []*alerting.Alert{}
	}
	writeJSON(w, http.StatusOK, alerts)
	return nil
}

func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}
	if !s.alertingManager.ResolveAlert(id) {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "resolved"})
	return nil
}

func (s *Server) handleAddNotificationGroup(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return badRequest("alerting manager not configured")
	}

	var req struct {
		Name     string   `json:"name"`
		Channels []string `json:"channels"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	group := &alerting.NotificationGroup{
		ID:       generateID(),
		Name:     req.Name,
		Channels: req.Channels,
	}

	s.alertingManager.AddNotificationGroup(group)
	writeJSON(w, http.StatusCreated, group)
	return nil
}
