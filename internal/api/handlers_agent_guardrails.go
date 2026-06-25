package api

import (
	"net/http"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func (s *Server) handleCreateAgentGuardrailConfig(w http.ResponseWriter, r *http.Request) error {
	agentID := r.PathValue("id")
	if _, err := s.db.GetAgent(r.Context(), agentID); err != nil {
		return ErrNotFound
	}

	var req struct {
		Name             string   `json:"name"`
		Enabled          bool     `json:"enabled"`
		MaxCostPerRun    float64  `json:"max_cost_per_run"`
		MaxLatencyMs     int64    `json:"max_latency_ms"`
		MaxTokensPerStep int      `json:"max_tokens_per_step"`
		ContentPolicy    []string `json:"content_policy"`
		RestrictedTerms  []string `json:"restricted_terms"`
		StopOnViolation  bool     `json:"stop_on_violation"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	now := time.Now()
	cfg := &models.AgentGuardrailConfig{
		ID:               generateID(),
		AgentID:          agentID,
		Name:             req.Name,
		Enabled:          req.Enabled,
		MaxCostPerRun:    req.MaxCostPerRun,
		MaxLatencyMs:     req.MaxLatencyMs,
		MaxTokensPerStep: req.MaxTokensPerStep,
		ContentPolicy:    req.ContentPolicy,
		RestrictedTerms:  req.RestrictedTerms,
		StopOnViolation:  req.StopOnViolation,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if cfg.ContentPolicy == nil {
		cfg.ContentPolicy = []string{}
	}
	if cfg.RestrictedTerms == nil {
		cfg.RestrictedTerms = []string{}
	}

	if err := s.db.SaveAgentGuardrailConfig(r.Context(), cfg); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "agent_guardrail_config:"+cfg.ID, map[string]any{"agent_id": agentID})
	writeJSON(w, http.StatusCreated, cfg)
	return nil
}

func (s *Server) handleGetAgentGuardrailConfig(w http.ResponseWriter, r *http.Request) error {
	agentID := r.PathValue("id")
	cfg, err := s.db.GetAgentGuardrailConfigByAgent(r.Context(), agentID)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, cfg)
	return nil
}

func (s *Server) handleUpdateAgentGuardrailConfig(w http.ResponseWriter, r *http.Request) error {
	configID := r.PathValue("config_id")
	existing, err := s.db.GetAgentGuardrailConfig(r.Context(), configID)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name             *string   `json:"name"`
		Enabled          *bool     `json:"enabled"`
		MaxCostPerRun    *float64  `json:"max_cost_per_run"`
		MaxLatencyMs     *int64    `json:"max_latency_ms"`
		MaxTokensPerStep *int      `json:"max_tokens_per_step"`
		ContentPolicy    *[]string `json:"content_policy"`
		RestrictedTerms  *[]string `json:"restricted_terms"`
		StopOnViolation  *bool     `json:"stop_on_violation"`
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
	if req.MaxCostPerRun != nil {
		existing.MaxCostPerRun = *req.MaxCostPerRun
	}
	if req.MaxLatencyMs != nil {
		existing.MaxLatencyMs = *req.MaxLatencyMs
	}
	if req.MaxTokensPerStep != nil {
		existing.MaxTokensPerStep = *req.MaxTokensPerStep
	}
	if req.ContentPolicy != nil {
		existing.ContentPolicy = *req.ContentPolicy
	}
	if req.RestrictedTerms != nil {
		existing.RestrictedTerms = *req.RestrictedTerms
	}
	if req.StopOnViolation != nil {
		existing.StopOnViolation = *req.StopOnViolation
	}

	existing.UpdatedAt = time.Now()

	if err := s.db.SaveAgentGuardrailConfig(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "agent_guardrail_config:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteAgentGuardrailConfig(w http.ResponseWriter, r *http.Request) error {
	configID := r.PathValue("config_id")
	if err := s.db.DeleteAgentGuardrailConfig(r.Context(), configID); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "agent_guardrail_config:"+configID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	return nil
}
