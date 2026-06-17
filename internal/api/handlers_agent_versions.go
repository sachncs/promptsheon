package api

import (
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/workflow"
)

func (s *Server) handleListAgentVersions(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	a, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	// Build version history from CAS if available
	versions := []map[string]any{
		{
			"version":    1,
			"cas_hash":   a.CASHash,
			"status":     a.Status,
			"created_at": a.CreatedAt,
			"updated_at": a.UpdatedAt,
			"created_by": a.CreatedBy,
		},
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id": id,
		"name":     a.Name,
		"versions": versions,
	})
	return nil
}

func (s *Server) handleRestoreAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Version int `json:"version"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// For now, just record the restore action
	// In full implementation, this would fetch from CAS store
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
		return err
	}

	s.auditDiff(r.Context(), "restore", "agent:"+existing.ID, nil, map[string]any{
		"restored_to_version": req.Version,
	})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeployAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	// Check status transition
	if existing.Status != models.StatusApproved {
		return badRequest("agent must be approved before deployment")
	}

	existing.Status = models.StatusDeployed
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
		return err
	}

	s.auditDiff(r.Context(), "deploy", "agent:"+existing.ID, models.StatusApproved, models.StatusDeployed)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleArchiveAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	existing.Status = models.StatusArchived
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
		return err
	}

	s.auditDiff(r.Context(), "archive", "agent:"+existing.ID, existing.Status, models.StatusArchived)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleRerunAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	a, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Input map[string]any `json:"input"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// Execute the workflow
	workflowFunc := func(agentID string, input map[string]any) (map[string]any, error) {
		return s.executeWorkflow(r.Context(), a, input)
	}

	output, err := workflowFunc(id, req.Input)
	if err != nil {
		return err
	}

	s.audit(r.Context(), "rerun", "agent:"+id, map[string]any{
		"input":  req.Input,
		"output": output,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id": id,
		"output":   output,
	})
	return nil
}

func (s *Server) executeWorkflow(ctx interface{}, a *models.Agent, input map[string]any) (map[string]any, error) {
	// Simplified workflow execution
	output := make(map[string]any)
	for _, step := range a.Steps {
		output[step.OutputKey] = input
	}
	return output, nil
}

func (s *Server) handleValidateAgentWorkflow(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Steps []models.AgentStep `json:"steps"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	validationErrors := workflow.ValidateSteps(req.Steps)
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":    len(validationErrors) == 0,
		"errors":   validationErrors,
	})
	return nil
}
