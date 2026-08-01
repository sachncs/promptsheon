package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"net/http"
	"time"

)

// Auto-split from handlers_capabilities.go
// ListWorkspaces lists the workspaces.
func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	workspaces, err := s.db.ListWorkspaces(r.Context())
	if err != nil {
		return err
	}
	if workspaces == nil {
		workspaces = []*capability.Workspace{}
	}
	paged := applyOffsetLimit(workspaces, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(workspaces), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

// CreateWorkspace creates the workspace.
// CreateWorkspace creates the workspace.
func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name         string `json:"name"`
		Organization string `json:"organization,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	now := time.Now()
	wksp := &capability.Workspace{
		ID:           generateID(),
		Name:         req.Name,
		Organization: req.Organization,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.CreateWorkspace(r.Context(), wksp); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "workspace:"+wksp.ID, map[string]any{KeyName: wksp.Name})
	writeJSON(w, http.StatusCreated, wksp)
	return nil
}

// GetWorkspace returns the workspace.
// GetWorkspace returns the workspace.
func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wksp, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return translateDBError(err, "workspace")
	}
	writeJSON(w, http.StatusOK, wksp)
	return nil
}

// UpdateWorkspace updates the workspace.
// UpdateWorkspace updates the workspace.
func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return translateDBError(err, "workspace")
	}
	var req struct {
		Name         *string `json:"name"`
		Organization *string `json:"organization,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Organization != nil {
		existing.Organization = *req.Organization
	}
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateWorkspace(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "workspace:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

// DeleteWorkspace deletes the workspace.
// DeleteWorkspace deletes the workspace.
func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteWorkspace(r.Context(), id); err != nil {
		return translateDBError(err, "workspace")
	}
	s.audit(r.Context(), "delete", "workspace:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
