package promptsheon

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/capability"
)

// Capability HTTP handlers (CRUD + self-evolve config).
// ListCapabilities lists the capabilities.
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

// CreateCapability creates the capability.
// CreateCapability creates the capability.
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
	s.audit(r.Context(), "create", "capability:"+capab.ID, map[string]any{KeyName: capab.Name, "project_id": projectID})
	writeJSON(w, http.StatusCreated, capab)
	return nil
}

// GetCapability returns the capability.
// GetCapability returns the capability.
func (s *Server) handleGetCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	writeJSON(w, http.StatusOK, c)
	return nil
}

// UpdateCapability updates the capability.
// UpdateCapability updates the capability.
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

// DeleteCapability deletes the capability.
// DeleteCapability deletes the capability.
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
// UpdateSelfEvolveConfig updates the selfEvolveConfig.
func (s *Server) handleUpdateSelfEvolveConfig(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	// ponytail: read into a map so we can tell "field present
	// with zero value" from "field omitted". A plain struct
	// decode makes both indistinguishable, which clobbered
	// every persisted threshold when the CLI sent
	// {"enabled": false}. The merged result preserves the
	// existing values for fields the client did not send.
	var raw map[string]json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		return ErrBadRequest
	}
	merged := existing.SelfEvolve
	if v, ok := raw["enabled"]; ok {
		if err := json.Unmarshal(v, &merged.Enabled); err != nil {
			return ErrBadRequest
		}
	}
	if v, ok := raw["min_score"]; ok {
		if err := json.Unmarshal(v, &merged.MinScore); err != nil {
			return ErrBadRequest
		}
	}
	if v, ok := raw["max_revisions"]; ok {
		if err := json.Unmarshal(v, &merged.MaxRevisions); err != nil {
			return ErrBadRequest
		}
	}
	if v, ok := raw["cooldown_sec"]; ok {
		if err := json.Unmarshal(v, &merged.CooldownSec); err != nil {
			return ErrBadRequest
		}
	}
	if v, ok := raw["target_env"]; ok {
		if err := json.Unmarshal(v, &merged.TargetEnv); err != nil {
			return ErrBadRequest
		}
	}
	if v, ok := raw["dataset_id"]; ok {
		if err := json.Unmarshal(v, &merged.DatasetID); err != nil {
			return ErrBadRequest
		}
	}
	if err := s.db.UpdateSelfEvolveConfig(r.Context(), existing.ID, merged); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "capability:"+existing.ID+":self_evolve", map[string]any{"enabled": merged.Enabled, "min_score": merged.MinScore, "max_revisions": merged.MaxRevisions})
	updated, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, updated)
	return nil
}
