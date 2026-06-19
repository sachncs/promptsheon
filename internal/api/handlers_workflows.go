package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/workflow"
)

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		AgentID string         `json:"agent_id"`
		Input   map[string]any `json:"input"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.AgentID == "" {
		return ErrBadRequest
	}

	// Add timeout to prevent indefinite execution
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	agent, err := s.db.GetAgent(ctx, req.AgentID)
	if err != nil {
		return ErrNotFound
	}

	registry := workflow.DefaultRegistry()
	engine := workflow.NewEngine(registry)

	result, err := engine.Execute(ctx, agent, req.Input)
	if err != nil {
		return err
	}

	// Convert to models.Workflow for persistence
	wf := &models.Workflow{
		ID:          result.WorkflowID,
		AgentID:     agent.ID,
		Status:      models.WorkflowStatus(result.Status),
		Input:       req.Input,
		Output:      result.Outputs,
		Error:       result.Error,
		StartedAt:   result.StartedAt,
		CompletedAt: &result.FinishedAt,
		CreatedAt:   result.StartedAt,
	}

	if err := s.db.SaveWorkflow(ctx, wf); err != nil {
		s.logger.Error("failed to save workflow", "err", err, "workflow_id", wf.ID)
		return err
	}

	// Persist individual step states
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
		if err := s.db.SaveWorkflowStep(ctx, step); err != nil {
			s.logger.Error("failed to save workflow step", "err", err, "step_id", stepID)
		}
	}

	s.audit(ctx, "workflow_run", "agent:"+agent.ID, map[string]any{
		"workflow_id": wf.ID,
		"status":      string(wf.Status),
		"steps":       len(result.Steps),
	})

	writeJSON(w, http.StatusOK, wf)
	return nil
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wf, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, wf)
	return nil
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) error {
	filter := models.WorkflowFilter{
		AgentID: r.URL.Query().Get("agent_id"),
		Status:  r.URL.Query().Get("status"),
		Limit:   50,
		Offset:  0,
	}

	// Parse limit parameter
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &filter.Limit); err == nil && n == 1 && filter.Limit > 0 && filter.Limit <= 1000 {
			// Use parsed value
		}
	}

	// Parse offset parameter
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &filter.Offset); err == nil && n == 1 && filter.Offset >= 0 {
			// Use parsed value
		}
	}

	workflows, err := s.db.ListWorkflows(r.Context(), filter)
	if err != nil {
		return err
	}
	if workflows == nil {
		workflows = []*models.Workflow{}
	}
	writeJSON(w, http.StatusOK, workflows)
	return nil
}

func (s *Server) handleGetWorkflowSteps(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	_, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	steps, err := s.db.GetWorkflowSteps(r.Context(), id)
	if err != nil {
		return err
	}
	if steps == nil {
		steps = []*models.WorkflowStep{}
	}
	writeJSON(w, http.StatusOK, steps)
	return nil
}

func (s *Server) handleCancelWorkflow(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wf, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	if wf.Status != models.WorkflowPending && wf.Status != models.WorkflowRunning {
		return ErrConflict
	}

	now := time.Now()
	wf.Status = models.WorkflowCancelled
	wf.CompletedAt = &now

	if err := s.db.SaveWorkflow(r.Context(), wf); err != nil {
		return err
	}

	s.audit(r.Context(), "workflow_cancel", "agent:"+wf.AgentID, map[string]any{
		"workflow_id": wf.ID,
	})

	writeJSON(w, http.StatusOK, wf)
	return nil
}
