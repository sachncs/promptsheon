package api

import (
	"net/http"
	"time"

	"github.com/sachn-cs/promptsheon/internal/abtesting"
	"github.com/sachn-cs/promptsheon/internal/llm"
)

func (s *Server) handleCreateABTest(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name        string              `json:"name"`
		PromptID    string              `json:"prompt_id"`
		Variants    []*abtesting.Variant `json:"variants"`
		WinCriteria string              `json:"win_criteria"`
		MinSamples  int                 `json:"min_samples"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if req.Name == "" || req.PromptID == "" {
		return badRequest("name and prompt_id are required")
	}
	
	if len(req.Variants) < 2 {
		return badRequest("at least 2 variants required")
	}
	
	if req.MinSamples <= 0 {
		req.MinSamples = 100
	}
	
	// Get provider
	p, err := s.db.GetPrompt(r.Context(), req.PromptID)
	if err != nil {
		return ErrNotFound
	}
	
	var providerName string
	if p.Binding != nil && p.Binding.Provider != "" {
		providerName = p.Binding.Provider
	} else {
		providerName = "openai"
	}
	
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available")
	}
	
	engine := abtesting.NewEngine(provider)
	test := &abtesting.Test{
		ID:          generateID(),
		Name:        req.Name,
		PromptID:    req.PromptID,
		Variants:    req.Variants,
		WinCriteria: req.WinCriteria,
		MinSamples:  req.MinSamples,
	}
	
	if err := engine.CreateTest(test); err != nil {
		return err
	}
	
	writeJSON(w, http.StatusCreated, test)
	return nil
}

func (s *Server) handleListABTests(w http.ResponseWriter, r *http.Request) error {
	// In production, would retrieve from database
	// For now, return empty list
	writeJSON(w, http.StatusOK, map[string]any{
		"tests": []any{},
		"total": 0,
	})
	return nil
}

func (s *Server) handleGetABTest(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	// Would retrieve from database in production
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"status": "not_found",
	})
	return nil
}

func (s *Server) handleStopABTest(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "test stopped",
		"id":      id,
	})
	return nil
}

func (s *Server) handleGetABTestResults(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"test_id":     id,
		"status":      "completed",
		"variants":    []any{},
		"winner":      nil,
		"confidence":  0,
		"is_significant": false,
		"timestamp":   time.Now(),
	})
	return nil
}
