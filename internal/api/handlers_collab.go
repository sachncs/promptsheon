package api

import (
	"net/http"

	"promptsheon/internal/collab"
)

var collabManager = collab.NewManager()

func (s *Server) handleCreateCollabSession(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PromptID string `json:"prompt_id"`
		Content  string `json:"content"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if req.PromptID == "" {
		return badRequest("prompt_id is required")
	}
	
	// Get prompt content if not provided
	if req.Content == "" {
		p, err := s.db.GetPrompt(r.Context(), req.PromptID)
		if err != nil {
			return ErrNotFound
		}
		req.Content = p.Content
	}
	
	session := collabManager.CreateSession(req.PromptID, req.Content)
	
	writeJSON(w, http.StatusCreated, session)
	return nil
}

func (s *Server) handleGetCollabSession(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	session, err := collabManager.GetSession(id)
	if err != nil {
		return ErrNotFound
	}
	
	writeJSON(w, http.StatusOK, session)
	return nil
}

func (s *Server) handleUpdateCursor(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	var req struct {
		UserID   string `json:"user_id"`
		Position int    `json:"position"`
		Color    string `json:"color"`
	}
	
	if err := readJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	
	if req.UserID == "" {
		return badRequest("user_id is required")
	}
	
	cursor := &collab.Cursor{
		UserID:   req.UserID,
		Position: req.Position,
		Color:    req.Color,
	}
	
	if err := collabManager.UpdateCursor(id, cursor); err != nil {
		return ErrNotFound
	}
	
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "cursor updated",
	})
	return nil
}

func (s *Server) handleGetChanges(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}
	
	sinceVersion := 0
	if v := r.URL.Query().Get("since"); v != "" {
		// Parse version number
		for _, c := range v {
			if c >= '0' && c <= '9' {
				sinceVersion = sinceVersion*10 + int(c-'0')
			}
		}
	}
	
	changes := collabManager.GetChanges(id, sinceVersion)
	
	writeJSON(w, http.StatusOK, map[string]any{
		"changes": changes,
		"total":   len(changes),
	})
	return nil
}
