package api

import (
	"encoding/json"
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/promptsheon"
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

	// If CAS hash exists, try to fetch parent commits for version history
	if a.CASHash != "" {
		obj, err := promptsheon.ReadObject(a.CASHash)
		if err == nil && obj.Parents != nil {
			for i, parentHash := range obj.Parents {
				parentObj, err := promptsheon.ReadObject(parentHash)
				if err == nil {
					versions = append(versions, map[string]any{
						"version":    i + 2,
						"cas_hash":   parentHash,
						"message":    parentObj.Message,
						"author":     parentObj.Author,
						"created_at": time.Unix(0, parentObj.Timestamp),
					})
				}
			}
		}
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
		Version int    `json:"version"`
		CASHash string `json:"cas_hash,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// If version is 1 or same as current, just return the current agent
	if req.Version == 1 || req.CASHash == existing.CASHash {
		writeJSON(w, http.StatusOK, existing)
		return nil
	}

	// If CAS hash is provided, use it directly
	// Otherwise, try to find the version from history
	var targetHash string
	if req.CASHash != "" {
		targetHash = req.CASHash
	} else if existing.CASHash != "" {
		// Try to find the version from parent commits
		obj, err := promptsheon.ReadObject(existing.CASHash)
		if err == nil && obj.Parents != nil {
			// Version 1 is the current, version 2+ are parents
			if req.Version > 1 && req.Version-2 < len(obj.Parents) {
				targetHash = obj.Parents[req.Version-2]
			}
		}
	}

	if targetHash == "" {
		return badRequest("invalid version or CAS hash")
	}

	// Fetch the object from CAS
	obj, err := promptsheon.ReadObject(targetHash)
	if err != nil {
		return badRequest("CAS object not found: " + targetHash)
	}

	// Parse the agent data from the CAS object
	if obj.Data != "" {
		var restoredAgent models.Agent
		if err := json.Unmarshal([]byte(obj.Data), &restoredAgent); err == nil {
			// Update the existing agent with restored data
			existing.Name = restoredAgent.Name
			existing.Description = restoredAgent.Description
			existing.Steps = restoredAgent.Steps
			existing.Tools = restoredAgent.Tools
			existing.Tags = restoredAgent.Tags
			existing.UpdatedAt = time.Now()
			existing.CASHash = targetHash

			if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
				return err
			}

			s.auditDiff(r.Context(), "restore", "agent:"+existing.ID, nil, map[string]any{
				"restored_to_version": req.Version,
				"cas_hash":            targetHash,
			})
			writeJSON(w, http.StatusOK, existing)
			return nil
		}
	}

	// If CAS object doesn't contain agent data, just update the hash
	existing.CASHash = targetHash
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
		return err
	}

	s.auditDiff(r.Context(), "restore", "agent:"+existing.ID, nil, map[string]any{
		"restored_to_version": req.Version,
		"cas_hash":            targetHash,
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
	if req.Input == nil {
		req.Input = make(map[string]any)
	}

	// Use the real workflow engine (simple re-execution, no guardrails or context)
	registry := workflow.DefaultRegistry()
	engine := workflow.NewEngine(registry)

	result, err := engine.Execute(r.Context(), a, req.Input)
	if err != nil {
		return err
	}

	// Persist workflow record
	wf := &models.Workflow{
		ID:          result.WorkflowID,
		AgentID:     a.ID,
		Status:      models.WorkflowStatus(result.Status),
		Input:       req.Input,
		Output:      result.Outputs,
		Error:       result.Error,
		StartedAt:   result.StartedAt,
		CompletedAt: &result.FinishedAt,
		CreatedAt:   result.StartedAt,
	}
	s.db.SaveWorkflow(r.Context(), wf)

	// Persist step results
	for stepID, sr := range result.Steps {
		startedAt := result.StartedAt
		finishedAt := result.FinishedAt
		step := &models.WorkflowStep{
			ID:         generateID(),
			WorkflowID: wf.ID,
			StepID:     stepID,
			Status:     string(sr.Status),
			Output:     sr.Output,
			Error:      sr.Error,
			ToolCalls:  sr.ToolCalls,
			LatencyMs:  sr.LatencyMs,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
		}
		s.db.SaveWorkflowStep(r.Context(), step)
	}

	s.audit(r.Context(), "rerun", "agent:"+id, map[string]any{
		"workflow_id": wf.ID,
		"status":      string(wf.Status),
		"steps":       len(result.Steps),
	})

	writeJSON(w, http.StatusOK, wf)
	return nil
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
		"valid":  len(validationErrors) == 0,
		"errors": validationErrors,
	})
	return nil
}
