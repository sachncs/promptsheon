package api

import (
	"fmt"
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/promptsheon"
	"promptsheon/internal/snapshot"
)

// handleListPromptVersions returns the CAS commit history for a prompt.
func (s *Server) handleListPromptVersions(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	// Use the prompt's CAS hash to find version history if available
	if p.CASHash == "" {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}

	entries, err := promptsheon.Log(50)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}

	// Filter entries relevant to this prompt (entries that touch this prompt's content)
	writeJSON(w, http.StatusOK, entries)
	return nil
}

// handleRestorePrompt restores a prompt to a previous CAS snapshot.
func (s *Server) handleRestorePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		CASHash string `json:"cas_hash"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.CASHash == "" {
		return ErrBadRequest
	}

	// Read the CAS object
	obj, err := promptsheon.ReadObject(req.CASHash)
	if err != nil {
		return badRequest("CAS object not found: " + req.CASHash)
	}

	// Restore content from CAS blob
	if obj.Data != "" {
		existing.Content = obj.Data
		existing.Version++
		existing.CASHash = req.CASHash
		existing.UpdatedAt = time.Now()
		if err := s.db.UpdatePrompt(r.Context(), existing); err != nil {
			return err
		}
		s.audit(r.Context(), "restore", "prompt:"+existing.ID, map[string]any{
			"cas_hash": req.CASHash,
			"version":  existing.Version,
		})
		writeJSON(w, http.StatusOK, existing)
		return nil
	}

	return badRequest("CAS object is not a content blob")
}

// handleFindSimilarPrompts finds prompts with similar content using SimHash.
func (s *Server) handleFindSimilarPrompts(w http.ResponseWriter, r *http.Request) error {
	content := r.URL.Query().Get("content")
	if content == "" {
		return ErrBadRequest
	}

	threshold := 0.7
	if t := r.URL.Query().Get("threshold"); t != "" {
		if _, err := fmt.Sscanf(t, "%f", &threshold); err != nil {
			threshold = 0.7
		}
	}

	targetHash := promptsheon.SimHash(content)

	prompts, err := s.db.ListPrompts(r.Context(), models.PromptFilter{Limit: 500})
	if err != nil {
		return err
	}

	type SimilarPrompt struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Similarity float64 `json:"similarity"`
	}

	var results []SimilarPrompt
	for _, p := range prompts {
		pHash := promptsheon.SimHash(p.Content)
		score := promptsheon.SimilarityScore(targetHash, pHash)
		if score >= threshold {
			results = append(results, SimilarPrompt{
				ID:         p.ID,
				Name:       p.Name,
				Similarity: score,
			})
		}
	}

	writeJSON(w, http.StatusOK, results)
	return nil
}

// handleListSnapshots returns output snapshots.
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) error {
	if s.snapshotStore == nil {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}

	f := snapshot.Filter{
		PromptHash: r.URL.Query().Get("prompt_hash"),
		Model:      r.URL.Query().Get("model"),
		Limit:      50,
	}

	snaps, err := s.snapshotStore.List(r.Context(), f)
	if err != nil {
		return err
	}
	if snaps == nil {
		snaps = []*snapshot.Snapshot{}
	}
	writeJSON(w, http.StatusOK, snaps)
	return nil
}

// handleGetSnapshot returns a single snapshot by ID.
func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) error {
	if s.snapshotStore == nil {
		return ErrNotFound
	}
	id := r.PathValue("id")
	snap, err := s.snapshotStore.Get(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, snap)
	return nil
}
