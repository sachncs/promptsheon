package api

import (
	"net/http"
	"time"

	"promptsheon/internal/models"
)

// handleForkAgent creates a new agent as a fork of an existing one.
func (s *Server) handleForkAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	original, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		req.Name = original.Name + " (fork)"
	}

	now := time.Now()
	forked := &models.Agent{
		ID:          generateID(),
		Name:        req.Name,
		Description: original.Description,
		Steps:       make([]models.AgentStep, len(original.Steps)),
		Tools:       make([]models.ToolRef, len(original.Tools)),
		Status:      models.StatusDraft,
		IsTemplate:  false,
		ParentID:    original.ID,
		CreatedBy:   callerID(r),
		Tags:        append([]string{}, original.Tags...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Deep copy steps
	for i, step := range original.Steps {
		forked.Steps[i] = models.AgentStep{
			ID:         step.ID,
			PromptID:   step.PromptID,
			PromptHash: step.PromptHash,
			DependsOn:  append([]string{}, step.DependsOn...),
			ToolCalls:  append([]models.ToolCall{}, step.ToolCalls...),
			OutputKey:  step.OutputKey,
			Condition:  step.Condition,
		}
	}

	// Deep copy tools
	for i, tool := range original.Tools {
		configCopy := make(map[string]any)
		for k, v := range tool.Config {
			configCopy[k] = v
		}
		forked.Tools[i] = models.ToolRef{
			Name:   tool.Name,
			Type:   tool.Type,
			Config: configCopy,
		}
	}

	if err := s.db.CreateAgent(r.Context(), forked); err != nil {
		return err
	}
	s.audit(r.Context(), "fork", "agent:"+forked.ID, map[string]any{
		"parent_id":   original.ID,
		"parent_name": original.Name,
	})
	writeJSON(w, http.StatusCreated, forked)
	return nil
}

// handleListTemplates returns all agents marked as templates.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) error {
	agents, err := s.db.ListAgents(r.Context())
	if err != nil {
		return err
	}

	var templates []*models.Agent
	for _, a := range agents {
		if a.IsTemplate {
			templates = append(templates, a)
		}
	}
	if templates == nil {
		templates = []*models.Agent{}
	}
	writeJSON(w, http.StatusOK, templates)
	return nil
}
