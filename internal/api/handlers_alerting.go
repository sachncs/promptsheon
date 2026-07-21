package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/alerting"
)

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	rules := s.alertingManager.ListRules()
	if rules == nil {
		rules = []*alerting.AlertRule{}
	}
	writeJSON(w, http.StatusOK, applyOffsetLimit(rules, offset, limit))
	return nil
}

func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}

	var req struct {
		Name      string         `json:"name"`
		Type      string         `json:"type"`
		Severity  string         `json:"severity"`
		Threshold float64        `json:"threshold"`
		Duration  int            `json:"duration_minutes"`
		Window    int            `json:"window_minutes"`
		Config    map[string]any `json:"config,omitempty"`
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
	s.audit(r.Context(), "create", "alert_rule:"+rule.ID, map[string]any{auditKeyName: rule.Name})
	writeJSON(w, http.StatusCreated, rule)
	return nil
}

func (s *Server) handleGetAlertRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
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
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}

	existing, ok := s.alertingManager.GetRule(id)
	if !ok {
		return ErrNotFound
	}

	var req struct {
		Name      *string        `json:"name"`
		Enabled   *bool          `json:"enabled"`
		Threshold *float64       `json:"threshold"`
		Config    map[string]any `json:"config,omitempty"`
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
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}
	// BUG-17: distinguish 204 (deleted) from 404 (never existed).
	// The previous form returned 204 unconditionally, so operators
	// could not tell whether their DELETE actually removed a row.
	if _, ok := s.alertingManager.GetRule(id); !ok {
		return ErrNotFound
	}
	s.alertingManager.RemoveRule(id)
	s.audit(r.Context(), "delete", "alert_rule:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	alerts := s.alertingManager.ListAlerts()
	if alerts == nil {
		alerts = []*alerting.Alert{}
	}
	writeJSON(w, http.StatusOK, applyOffsetLimit(alerts, offset, limit))
	return nil
}

func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
	}
	if !s.alertingManager.ResolveAlert(id) {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, map[string]any{auditKeyStatus: "resolved"})
	return nil
}

func (s *Server) handleAddNotificationGroup(w http.ResponseWriter, r *http.Request) error {
	if s.alertingManager == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "alerting manager not configured"}
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
	s.audit(r.Context(), "notification_group_create", "notification_group:"+group.ID, map[string]any{
		"name":     group.Name,
		"channels": group.Channels,
	})
	writeJSON(w, http.StatusCreated, group)
	return nil
}

// handleLinkAlertRuleGroup wires an alert rule to a notification
// group. DB-11b: closes the loop so operators don't have to write
// to alert_rule_notification_groups by hand.
func (s *Server) handleLinkAlertRuleGroup(w http.ResponseWriter, r *http.Request) error {
	ruleID := r.PathValue("rule_id")
	groupID := r.PathValue("group_id")
	if ruleID == "" || groupID == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "rule_id and group_id are required"}
	}
	if err := s.db.LinkRuleToGroup(r.Context(), ruleID, groupID); err != nil {
		return fmt.Errorf("link rule to group: %w", err)
	}
	s.audit(r.Context(), "alert_link", "alert_rule:"+ruleID, map[string]any{
		"notification_group_id": groupID,
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// handleUnlinkAlertRuleGroup removes the wire between an alert
// rule and a notification group. DB-11b.
func (s *Server) handleUnlinkAlertRuleGroup(w http.ResponseWriter, r *http.Request) error {
	ruleID := r.PathValue("rule_id")
	groupID := r.PathValue("group_id")
	if ruleID == "" || groupID == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "rule_id and group_id are required"}
	}
	if err := s.db.UnlinkRuleFromGroup(r.Context(), ruleID, groupID); err != nil {
		return fmt.Errorf("unlink rule from group: %w", err)
	}
	s.audit(r.Context(), "alert_unlink", "alert_rule:"+ruleID, map[string]any{
		"notification_group_id": groupID,
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
