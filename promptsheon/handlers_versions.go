package promptsheon

import (
	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// Auto-split from handlers_capabilities.go

// handleListVersions returns every immutable Version for the
// given Capability, ordered by version number (newest first).
// ListVersions lists the versions.
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	capabilityID := r.PathValue("capability_id")
	versions, err := s.db.ListVersions(r.Context(), capabilityID)
	if err != nil {
		return err
	}
	if versions == nil {
		versions = []*capability.Version{}
	}
	paged := applyOffsetLimit(versions, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(versions), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

// CreateVersion creates the version.
func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	var req struct {
		Version  int                 `json:"version"`
		Manifest capability.Manifest `json:"manifest"`
		Parents  []string            `json:"parents,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	// API-VAL-2: reject non-positive version numbers so the
	// caller can't insert a phantom "v0" or "v-1".
	if err := validatePositiveInt("version", req.Version); err != nil {
		return err
	}
	// Forward-only: every request MUST supply a Manifest. The
	// legacy synthesis helper is gone; clients that still pass
	// the old bundle shape get 400 with a manifest-required error.
	manifest := req.Manifest
	if err := manifest.Validate(); err != nil {
		return badRequest("manifest: " + err.Error())
	}
	// INHERIT-1 wiring: if the request declares Parents, the
	// child's Manifest is overridden by the parent's artifacts
	// for every Kind the parent supplies. Cycles and depth
	// overflow are surfaced as 422 (Unprocessable Entity).
	if len(req.Parents) > 0 {
		resolver := &versionResolverAdapter{repo: s.db}
		merged, err := capability.ResolveManifest(&capability.Version{
			ID:           "",
			CapabilityID: capabilityID,
			Version:      req.Version,
			Manifest:     manifest,
			Parents:      req.Parents,
		}, resolver)
		if err != nil {
			var cycle *capability.ErrInheritanceCycle
			if errors.As(err, &cycle) {
				return &HTTPError{Status: http.StatusUnprocessableEntity, Message: err.Error()}
			}
			if errors.Is(err, errs.ErrInheritanceTooDeep) {
				return &HTTPError{Status: http.StatusUnprocessableEntity, Message: err.Error()}
			}
			return &HTTPError{Status: http.StatusBadRequest, Message: err.Error()}
		}
		manifest = merged
	}
	hash, err := capability.ComputeManifestHash(manifest)
	if err != nil {
		return errf.Errorf("compute manifest hash: %w", err)
	}
	now := time.Now()
	v := &capability.Version{
		ID:           generateID(),
		CapabilityID: capabilityID,
		Version:      req.Version,
		Manifest:     manifest,
		ManifestHash: hash,
		Parents:      req.Parents,
		CreatedAt:    now,
		CreatedBy:    callerID(r),
	}
	if err := s.db.CreateVersion(r.Context(), v); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "version:"+v.ID, map[string]any{"capability_id": capabilityID, KeyVersion: v.Version, "manifest_hash": hash, "parents": req.Parents})
	writeJSON(w, http.StatusCreated, v)
	return nil
}

// versionResolverAdapter adapts the SQLite Repository surface
// to the InheritanceResolver contract used by ResolveManifest.
type versionResolverAdapter struct {
	repo capability.Repository
}

func (a *versionResolverAdapter) GetVersion(id string) (*capability.Version, error) {
	return a.repo.GetVersion(context.Background(), id)
}

// GetVersion returns the version.
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	v, err := s.db.GetVersion(r.Context(), id)
	if err != nil {
		return translateDBError(err, "version")
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

// GetLatestVersion returns the latestVersion.
func (s *Server) handleGetLatestVersion(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	v, err := s.db.GetLatestVersion(r.Context(), capabilityID)
	if err != nil {
		return translateDBError(err, "version")
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}
