package backend

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
)

// Capability HTTP handlers (CRUD + self-evolve config).

func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	projectID := r.PathValue("project_id")
	caps, err := s.db.ListCapabilities(r.Context(), projectID)
	if err != nil {
		return err
	}
	if caps == nil {
		caps = []*capability.Capability{}
	}
	paged := applyOffsetLimit(caps, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(caps), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

func (s *Server) handleCreateCapability(w http.ResponseWriter, r *http.Request) error {
	projectID := r.PathValue("project_id")
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Owner       string   `json:"owner,omitempty"`
		Tags        []string `json:"tags,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	// API-VAL-3: Owner must reference an existing user when
	// supplied. An empty Owner is allowed (means "no owner").
	if req.Owner != "" {
		if _, err := s.db.GetUser(r.Context(), req.Owner); err != nil {
			return badRequest("owner: " + translateDBError(err, "user").Error())
		}
	}
	now := time.Now()
	capab := &capability.Capability{
		ID:          generateID(),
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		Owner:       req.Owner,
		Tags:        req.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.CreateCapability(r.Context(), capab); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "capability:"+capab.ID, map[string]any{auditKeyName: capab.Name, "project_id": projectID})
	writeJSON(w, http.StatusCreated, capab)
	return nil
}

func (s *Server) handleGetCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	writeJSON(w, http.StatusOK, c)
	return nil
}

func (s *Server) handleUpdateCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	var req struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description,omitempty"`
		Owner       *string   `json:"owner,omitempty"`
		Tags        *[]string `json:"tags,omitempty"`
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
	if req.Owner != nil {
		existing.Owner = *req.Owner
	}
	if req.Tags != nil {
		existing.Tags = *req.Tags
	}
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateCapability(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "capability:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteCapability(r.Context(), id); err != nil {
		return translateDBError(err, "capability")
	}
	s.audit(r.Context(), "delete", "capability:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// handleUpdateSelfEvolveConfig is the API backing the
// `promptsheon selfevolve` CLI subcommand. The body is a
// partial capability.SelfEvolveConfig; the daemon merges
// over the persisted config and persists. Operators flip
// the loop on/off here without restarting the daemon.
func (s *Server) handleUpdateSelfEvolveConfig(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	var req capability.SelfEvolveConfig
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := s.db.UpdateSelfEvolveConfig(r.Context(), existing.ID, req); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "capability:"+existing.ID+":self_evolve", map[string]any{"enabled": req.Enabled, "min_score": req.MinScore, "max_revisions": req.MaxRevisions})
	updated, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, updated)
	return nil
}

