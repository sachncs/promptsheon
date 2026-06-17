package api

import (
	"net/http"
	"time"

	"promptsheon/internal/models"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) error {
	users, err := s.db.ListUsers(r.Context())
	if err != nil {
		return err
	}
	if users == nil {
		users = []*models.User{}
	}
	writeJSON(w, http.StatusOK, users)
	return nil
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Email == "" || req.Name == "" {
		return ErrBadRequest
	}
	if req.Role == "" {
		req.Role = "reader"
	}

	now := time.Now()
	u := &models.User{
		ID:        generateID(),
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.db.CreateUser(r.Context(), u); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "user:"+u.ID, map[string]any{"email": u.Email, "role": u.Role})
	writeJSON(w, http.StatusCreated, u)
	return nil
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	u, err := s.db.GetUser(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, u)
	return nil
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetUser(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Email *string `json:"email"`
		Name  *string `json:"name"`
		Role  *string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Email != nil {
		existing.Email = *req.Email
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Role != nil {
		existing.Role = *req.Role
	}
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateUser(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "user:"+existing.ID, map[string]any{"email": existing.Email, "role": existing.Role})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteUser(r.Context(), id); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "user:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
