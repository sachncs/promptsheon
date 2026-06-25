package api

import (
	"net/http"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/playground"
)

func (s *Server) handlePlaygroundRun(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PromptID    string            `json:"prompt_id,omitempty"`
		Content     string            `json:"content"`
		Variables   map[string]string `json:"variables"`
		Model       string            `json:"model"`
		SystemMsg   string            `json:"system_message,omitempty"`
		MaxTokens   int               `json:"max_tokens"`
		Temperature float64           `json:"temperature"`
		Provider    string            `json:"provider"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if req.Content == "" && req.PromptID == "" {
		return badRequest("content or prompt_id is required")
	}
	
	// Get provider
	providerName := req.Provider
	if providerName == "" {
		providerName = "openai"
	}
	
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available: " + providerName)
	}
	
	pg := playground.NewPlayground()
	
	// If prompt_id provided, fetch content
	if req.PromptID != "" {
		p, err := s.db.GetPrompt(r.Context(), req.PromptID)
		if err != nil {
			return ErrNotFound
		}
		req.Content = p.Content
	}
	
	runReq := &playground.RunRequest{
		Content:     req.Content,
		Variables:   req.Variables,
		Model:       req.Model,
		SystemMsg:   req.SystemMsg,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	
	resp, err := pg.Run(r.Context(), provider, runReq)
	if err != nil {
		return err
	}
	
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) handlePlaygroundCompare(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Prompts   []playground.ComparePrompt `json:"prompts"`
		Model     string                     `json:"model"`
		Variables map[string]string          `json:"variables"`
		Provider  string                     `json:"provider"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if len(req.Prompts) < 2 {
		return badRequest("at least 2 prompts required for comparison")
	}
	
	providerName := req.Provider
	if providerName == "" {
		providerName = "openai"
	}
	
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available")
	}
	
	pg := playground.NewPlayground()
	
	compareReq := &playground.CompareRequest{
		Prompts:   req.Prompts,
		Model:     req.Model,
		Variables: req.Variables,
	}
	
	results, err := pg.Compare(r.Context(), provider, compareReq)
	if err != nil {
		return err
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"total":   len(results),
	})
	return nil
}

func (s *Server) handlePlaygroundTemplates(w http.ResponseWriter, r *http.Request) error {
	category := r.URL.Query().Get("category")
	
	pg := playground.NewPlayground()
	
	var templates []*playground.Template
	if category != "" {
		templates = pg.GetTemplatesByCategory(category)
	} else {
		templates = pg.GetTemplates()
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"templates": templates,
		"total":     len(templates),
	})
	return nil
}
