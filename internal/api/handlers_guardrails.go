package api

import (
	"context"
	"net/http"
	"time"

	"promptsheon/internal/auth"
	"promptsheon/internal/guardrail"
)

func (s *Server) handleListGuardrailRules(w http.ResponseWriter, r *http.Request) error {
	if s.guardrailManager == nil {
		writeJSON(w, http.StatusOK, []*guardrail.Rule{})
		return nil
	}
	rules := s.guardrailManager.ListRules()
	if rules == nil {
		rules = []*guardrail.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
	return nil
}

func (s *Server) handleCreateGuardrailRule(w http.ResponseWriter, r *http.Request) error {
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}

	var req struct {
		Name         string            `json:"name"`
		Type         string            `json:"type"`
		Severity     string            `json:"severity"`
		Config       map[string]any    `json:"config,omitempty"`
		Environments []string          `json:"environments,omitempty"`
		PromptIDs    []string          `json:"prompt_ids,omitempty"`
		AgentIDs     []string          `json:"agent_ids,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name == "" || req.Type == "" {
		return badRequest("name and type are required")
	}

	now := time.Now()
	rule := &guardrail.Rule{
		ID:           generateID(),
		Name:         req.Name,
		Type:         guardrail.ViolationType(req.Type),
		Severity:     guardrail.Severity(req.Severity),
		Enabled:      true,
		Config:       req.Config,
		Environments: req.Environments,
		PromptIDs:    req.PromptIDs,
		AgentIDs:     req.AgentIDs,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.guardrailManager.AddRule(rule)
	s.audit(r.Context(), "create", "guardrail_rule:"+rule.ID, map[string]any{"name": rule.Name})
	writeJSON(w, http.StatusCreated, rule)
	return nil
}

func (s *Server) handleGetGuardrailRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}
	rule, ok := s.guardrailManager.GetRule(id)
	if !ok {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, rule)
	return nil
}

func (s *Server) handleUpdateGuardrailRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}

	existing, ok := s.guardrailManager.GetRule(id)
	if !ok {
		return ErrNotFound
	}

	var req struct {
		Name         *string           `json:"name"`
		Enabled      *bool             `json:"enabled"`
		Config       map[string]any    `json:"config,omitempty"`
		Environments []string          `json:"environments,omitempty"`
		PromptIDs    []string          `json:"prompt_ids,omitempty"`
		AgentIDs     []string          `json:"agent_ids,omitempty"`
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
	if req.Config != nil {
		existing.Config = req.Config
	}
	if req.Environments != nil {
		existing.Environments = req.Environments
	}
	if req.PromptIDs != nil {
		existing.PromptIDs = req.PromptIDs
	}
	if req.AgentIDs != nil {
		existing.AgentIDs = req.AgentIDs
	}
	existing.UpdatedAt = time.Now()

	s.guardrailManager.AddRule(existing)
	s.audit(r.Context(), "update", "guardrail_rule:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteGuardrailRule(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}
	s.guardrailManager.RemoveRule(id)
	s.audit(r.Context(), "delete", "guardrail_rule:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListGuardrailViolations(w http.ResponseWriter, r *http.Request) error {
	if s.guardrailManager == nil {
		writeJSON(w, http.StatusOK, []*guardrail.Violation{})
		return nil
	}
	violations := s.guardrailManager.ListViolations()
	if violations == nil {
		violations = []*guardrail.Violation{}
	}
	writeJSON(w, http.StatusOK, violations)
	return nil
}

func (s *Server) handleResolveGuardrailViolation(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}

	violations := s.guardrailManager.ListViolations()
	for _, v := range violations {
		if v.ID == id {
			v.Resolved = true
			if u, ok := getUserFromContext(r.Context()); ok {
				v.ResolvedBy = u
			}
			now := time.Now()
			v.ResolvedAt = &now
			writeJSON(w, http.StatusOK, v)
			return nil
		}
	}
	return ErrNotFound
}

func (s *Server) handleCheckGuardrails(w http.ResponseWriter, r *http.Request) error {
	if s.guardrailManager == nil {
		return badRequest("guardrail manager not configured")
	}

	var req struct {
		Content     string `json:"content"`
		Model       string `json:"model"`
		Environment string `json:"environment"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	violations := s.guardrailManager.RunAllStaticChecks(r.Context(), req.Content, req.Model, req.Environment)
	writeJSON(w, http.StatusOK, map[string]any{
		"passed":     len(violations) == 0,
		"violations": violations,
	})
	return nil
}

func getUserFromContext(ctx context.Context) (string, bool) {
	if u, ok := auth.UserFromContext(ctx); ok {
		return u.ID, true
	}
	return "", false
}
