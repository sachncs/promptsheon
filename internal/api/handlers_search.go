package api

import (
	"net/http"

	"promptsheon/internal/models"
	"promptsheon/internal/search"
)

func (s *Server) handleSemanticSearch(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if req.Query == "" {
		return badRequest("query is required")
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	
	// Get prompts from database
	prompts, err := s.db.ListPrompts(r.Context(), models.PromptFilter{})
	if err != nil {
		return err
	}
	
	// Index prompts
	indexer := search.NewIndexer()
	for _, p := range prompts {
		indexer.IndexPrompt(p)
	}
	
	// Search
	results := indexer.SearchPrompts(req.Query, req.Limit)
	
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"total":   len(results),
	})
	return nil
}

func (s *Server) handleIndexPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	
	indexer := search.NewIndexer()
	indexer.IndexPrompt(p)
	
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "prompt indexed successfully",
		"stats":   indexer.GetIndexStats(),
	})
	return nil
}

func (s *Server) handleFindSimilar(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	
	// Get all prompts
	prompts, err := s.db.ListPrompts(r.Context(), models.PromptFilter{})
	if err != nil {
		return err
	}
	
	// Index and search
	indexer := search.NewIndexer()
	for _, prompt := range prompts {
		indexer.IndexPrompt(prompt)
	}
	
	// Use the prompt content as search query
	results := indexer.SearchPrompts(p.Content, 10)
	
	// Filter out the source prompt
	filtered := make([]*search.SearchResult, 0)
	for _, r := range results {
		if r.Document.PromptID != id {
			filtered = append(filtered, r)
		}
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"source":  p.ID,
		"results": filtered,
		"total":   len(filtered),
	})
	return nil
}
