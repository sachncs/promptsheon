package backend

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
)

// Auto-split from handlers_capabilities.go

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	workspaceID := r.PathValue("workspace_id")
	projects, err := s.db.ListProjects(r.Context(), workspaceID)
	if err != nil {
		return err
	}
	if projects == nil {
		projects = []*capability.Project{}
	}
	paged := applyOffsetLimit(projects, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(projects), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

// CreateProject creates the project.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) error {
	workspaceID := r.PathValue("workspace_id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	now := time.Now()
	proj := &capability.Project{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.CreateProject(r.Context(), proj); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "project:"+proj.ID, map[string]any{auditKeyName: proj.Name, "workspace_id": workspaceID})
	writeJSON(w, http.StatusCreated, proj)
	return nil
}

// GetProject returns the project.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	proj, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return translateDBError(err, "project")
	}
	writeJSON(w, http.StatusOK, proj)
	return nil
}

// UpdateProject updates the project.
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return translateDBError(err, "project")
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description,omitempty"`
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
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateProject(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "project:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

// DeleteProject deletes the project.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteProject(r.Context(), id); err != nil {
		return translateDBError(err, "project")
	}
	s.audit(r.Context(), "delete", "project:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

