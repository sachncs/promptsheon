package api

import (
	"net/http"
	"time"

	"promptsheon/internal/models"
)

func (s *Server) handleListContexts(w http.ResponseWriter, r *http.Request) error {
	filter := models.ContextFilter{
		AgentID: r.URL.Query().Get("agent_id"),
		Type:    models.ContextType(r.URL.Query().Get("type")),
		Search:  r.URL.Query().Get("search"),
		Limit:   50,
	}

	contexts, err := s.db.ListContexts(r.Context(), filter)
	if err != nil {
		return err
	}
	if contexts == nil {
		contexts = []*models.Context{}
	}
	writeJSON(w, http.StatusOK, contexts)
	return nil
}

func (s *Server) handleCreateContext(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name               string                    `json:"name"`
		Description        string                    `json:"description"`
		Type               models.ContextType        `json:"type"`
		SystemPrompt       string                    `json:"system_prompt"`
		Messages           []models.ContextMessage   `json:"messages"`
		TokenBudget        int                       `json:"token_budget"`
		TruncationStrategy models.TruncationStrategy `json:"truncation_strategy"`
		AgentID            string                    `json:"agent_id"`
		Metadata           map[string]string         `json:"metadata"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return badRequest("name is required")
	}

	now := time.Now()
	c := &models.Context{
		ID:                 generateID(),
		Name:               req.Name,
		Description:        req.Description,
		Type:               req.Type,
		SystemPrompt:       req.SystemPrompt,
		Messages:           req.Messages,
		TokenBudget:        req.TokenBudget,
		TruncationStrategy: req.TruncationStrategy,
		AgentID:            req.AgentID,
		Version:            1,
		Status:             models.StatusDraft,
		Metadata:           req.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if c.Type == "" {
		c.Type = models.ContextSystemPrompt
	}
	if c.TruncationStrategy == "" {
		c.TruncationStrategy = models.TruncationSlidingWindow
	}
	if c.TokenBudget == 0 {
		c.TokenBudget = 4096
	}
	if c.Messages == nil {
		c.Messages = []models.ContextMessage{}
	}

	// Count initial tokens
	if s.contextManager != nil {
		c.TokenCount = s.contextManager.EstimateTokens(c.SystemPrompt)
		for _, msg := range c.Messages {
			c.TokenCount += s.contextManager.EstimateTokens(msg.Content)
		}
	}

	if err := s.db.CreateContext(r.Context(), c); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "context:"+c.ID, map[string]any{"name": c.Name})
	writeJSON(w, http.StatusCreated, c)
	return nil
}

func (s *Server) handleGetContext(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetContext(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, c)
	return nil
}

func (s *Server) handleUpdateContext(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetContext(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name               *string                    `json:"name"`
		Description        *string                    `json:"description"`
		Type               *models.ContextType        `json:"type"`
		SystemPrompt       *string                    `json:"system_prompt"`
		Messages           *[]models.ContextMessage   `json:"messages"`
		TokenBudget        *int                       `json:"token_budget"`
		TruncationStrategy *models.TruncationStrategy `json:"truncation_strategy"`
		Status             *models.PromptStatus       `json:"status"`
		Metadata           *map[string]string         `json:"metadata"`
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
	if req.Type != nil {
		existing.Type = *req.Type
	}
	if req.SystemPrompt != nil {
		existing.SystemPrompt = *req.SystemPrompt
	}
	if req.Messages != nil {
		existing.Messages = *req.Messages
	}
	if req.TokenBudget != nil {
		existing.TokenBudget = *req.TokenBudget
	}
	if req.TruncationStrategy != nil {
		existing.TruncationStrategy = *req.TruncationStrategy
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Metadata != nil {
		existing.Metadata = *req.Metadata
	}

	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateContext(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "context:"+existing.ID, map[string]any{"name": existing.Name})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteContext(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteContext(r.Context(), id); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "context:"+id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	return nil
}

func (s *Server) handleAppendContextMessage(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetContext(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Role == "" || req.Content == "" {
		return badRequest("role and content are required")
	}

	msg := models.ContextMessage{
		ID:        generateID(),
		Role:      req.Role,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	// Estimate tokens
	if s.contextManager != nil {
		msg.TokenCount = s.contextManager.EstimateTokens(req.Content)
	}

	c.Messages = append(c.Messages, msg)
	c.TokenCount += msg.TokenCount
	c.UpdatedAt = time.Now()

	if err := s.db.UpdateContext(r.Context(), c); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, msg)
	return nil
}

func (s *Server) handleClearContextMessages(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetContext(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	c.Messages = []models.ContextMessage{}
	// Recalculate token count from system prompt only
	if s.contextManager != nil {
		c.TokenCount = s.contextManager.EstimateTokens(c.SystemPrompt)
	} else {
		c.TokenCount = 0
	}
	c.UpdatedAt = time.Now()

	if err := s.db.UpdateContext(r.Context(), c); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"cleared": true})
	return nil
}

func (s *Server) handleAssembleContext(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	var req struct {
		Variables map[string]string `json:"variables"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if s.contextManager == nil {
		return badRequest("context manager not configured")
	}

	assembled, err := s.contextManager.Assemble(r.Context(), id, req.Variables)
	if err != nil {
		return ErrNotFound
	}

	writeJSON(w, http.StatusOK, assembled)
	return nil
}
