package api

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/models"
)

const roleReader = "reader"
const fieldEmail = "email"
const fieldRole = "role"

// knownRoles is the closed set of valid user roles. Accepting
// anything outside this set lets a caller grant themselves an
// ad-hoc role (e.g. "superuser") that no downstream code maps
// to a permission set — a privilege-escalation foot-gun.
var knownRoles = map[string]struct{}{
	string(auth.RoleAdmin):  {},
	string(auth.RoleWriter): {},
	string(auth.RoleReader): {},
}

func validRole(r string) bool {
	_, ok := knownRoles[r]
	return ok
}

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
	if err := validateNonEmpty("email", req.Email); err != nil {
		return err
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	// API-VAL-6: enforce RFC 5322 syntax. The previous form
	// accepted any string with an "@" in it; tightening this
	// blocks obvious typos and invalid addresses that the
	// downstream OAuth flows would reject anyway.
	if !validEmail(req.Email) {
		return badRequest("email is not a valid address")
	}
	if req.Role == "" {
		req.Role = roleReader
	}
	if !validRole(req.Role) {
		return badRequest("role must be one of admin, writer, reader")
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
	s.audit(r.Context(), "create", "user:"+u.ID, map[string]any{fieldEmail: u.Email, fieldRole: u.Role})
	writeJSON(w, http.StatusCreated, u)
	return nil
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	u, err := s.db.GetUser(r.Context(), id)
	if err != nil {
		return translateDBError(err, "user")
	}
	writeJSON(w, http.StatusOK, u)
	return nil
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetUser(r.Context(), id)
	if err != nil {
		return translateDBError(err, "user")
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
		// API-VAL-6: same email-shape check on update. A typo
		// here is harder to spot than on create (no GET-then-PUT
		// round-trip in the standard flow).
		if !validEmail(*req.Email) {
			return badRequest("email is not a valid address")
		}
		existing.Email = *req.Email
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Role != nil {
		if !validRole(*req.Role) {
			return badRequest("role must be one of admin, writer, reader")
		}
		existing.Role = *req.Role
	}
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdateUser(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "user:"+existing.ID, map[string]any{fieldEmail: existing.Email, fieldRole: existing.Role})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteUser(r.Context(), id); err != nil {
		return translateDBError(err, "user")
	}
	s.audit(r.Context(), "delete", "user:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
