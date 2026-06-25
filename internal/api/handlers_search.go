package api

import (
	"net/http"

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
	// L-3 fix: handlers_search.go is now gofmt-clean and the
	// previous trailing-whitespace drift is removed. The change
	// in this commit is whitespace only; functionality is
	// unchanged.

	if req.Query == "" {
		return badRequest("query is required")
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// M-1 fix: use the server-owned in-memory index instead of
	// re-indexing every prompt on every request. The previous
	// implementation called ListPrompts + rebuilt the index +
	// searched, which was O(N) per request.
	results := s.searchManager.Search(req.Query, req.Limit)

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

	// M-1 fix: add to the server-owned index. The previous
	// implementation created a throwaway Indexer instance that
	// was discarded after the request.
	s.searchManager.Add(p)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "prompt indexed successfully",
		"stats":   map[string]int{"total_documents": s.searchManager.Size()},
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

	// M-1 fix: search the server-owned index. The previous
	// implementation re-indexed every prompt on every request.
	results := s.searchManager.Search(p.Content, 10)

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
