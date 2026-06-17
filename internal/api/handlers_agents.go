package api

import (
	"io"
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/workflow"
)

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) error {
	agents, err := s.db.ListAgents(r.Context())
	if err != nil {
		return err
	}
	if agents == nil {
		agents = []*models.Agent{}
	}
	writeJSON(w, http.StatusOK, agents)
	return nil
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Steps       []models.AgentStep `json:"steps"`
		Tools       []models.ToolRef   `json:"tools"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return ErrBadRequest
	}

	// Validate DAG before saving
	if validationErrors := workflow.ValidateSteps(req.Steps); len(validationErrors) > 0 {
		return badRequestf("workflow validation failed: %v", validationErrors)
	}

	now := time.Now()
	a := &models.Agent{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Steps:       req.Steps,
		Tools:       req.Tools,
		Status:      models.StatusDraft,
		CreatedBy:   "api",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.db.CreateAgent(r.Context(), a); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "agent:"+a.ID, map[string]any{"name": a.Name})
	writeJSON(w, http.StatusCreated, a)
	return nil
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	a, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, a)
	return nil
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name        *string            `json:"name"`
		Description *string            `json:"description"`
		Steps       []models.AgentStep `json:"steps"`
		Tools       []models.ToolRef   `json:"tools"`
		Status      *string            `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Steps != nil {
		// Validate DAG before saving
		if validationErrors := workflow.ValidateSteps(req.Steps); len(validationErrors) > 0 {
			return badRequestf("workflow validation failed: %v", validationErrors)
		}
		existing.Steps = req.Steps
	}
	if req.Tools != nil {
		existing.Tools = req.Tools
	}
	if req.Status != nil {
		newStatus := models.PromptStatus(*req.Status)
		// Enforce review-gated transitions: cannot directly set approved without a review
		if newStatus == models.StatusApproved && existing.Status != models.StatusApproved {
			reviews, _ := s.db.ListReviewsByResource(r.Context(), existing.ID, "agent")
			approvedCount := 0
			quorumRequired := 1
			for _, rv := range reviews {
				if rv.Status == models.ReviewApproved {
					approvedCount++
				}
				if rv.QuorumRequired > quorumRequired {
					quorumRequired = rv.QuorumRequired
				}
			}
			if approvedCount < quorumRequired {
				return badRequestf("cannot approve agent: need %d approvals, have %d", quorumRequired, approvedCount)
			}
		}
		s.auditDiff(r.Context(), "update_status", "agent:"+existing.ID, existing.Status, newStatus)
		existing.Status = newStatus
	}
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateAgent(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "agent:"+existing.ID, map[string]any{"name": existing.Name})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteAgent(r.Context(), id); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "agent:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleExportAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	a, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	if r.URL.Query().Get("format") == "yaml" {
		yamlData, err := workflow.ExportYAML(a)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(yamlData)
		return nil
	}

	writeJSON(w, http.StatusOK, a)
	return nil
}

func (s *Server) handleImportAgentYAML(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ErrBadRequest
	}
	defer r.Body.Close()

	agent, err := workflow.ParseYAML(body)
	if err != nil {
		return badRequestf("invalid YAML: %v", err)
	}

	// Validate DAG before saving
	if validationErrors := workflow.ValidateSteps(agent.Steps); len(validationErrors) > 0 {
		return badRequestf("workflow validation failed: %v", validationErrors)
	}

	now := time.Now()
	agent.ID = generateID()
	agent.Status = models.StatusDraft
	agent.CreatedBy = "api"
	agent.CreatedAt = now
	agent.UpdatedAt = now

	if err := s.db.CreateAgent(r.Context(), agent); err != nil {
		return err
	}
	s.audit(r.Context(), "import_yaml", "agent:"+agent.ID, map[string]any{"name": agent.Name})
	writeJSON(w, http.StatusCreated, agent)
	return nil
}
