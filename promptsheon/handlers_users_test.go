
package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/models"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestHandleListUsers(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "admin"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateUser(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"email": "new@test.com", "name": "New User", "role": "reader"})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["email"] != "new@test.com" {
		t.Errorf("expected new@test.com, got %v", resp["email"])
	}
}

func TestHandleCreateUser_MissingFields(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"email": "new@test.com"})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateUser_RejectsInvalidRole(t *testing.T) {
	// API-VAL-6: reject roles outside the closed admin/writer/reader set.
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{
		"email": "u@test.com", "name": "U", "role": "superuser",
	})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown role, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateUser_RejectsInvalidRole(t *testing.T) {
	s := newTestServer(t)
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "reader"})
	s.db = newDB(repo)

	body := mustMarshal(t, map[string]string{"role": "superuser"})
	req := httptest.NewRequest("PUT", "/api/v1/users/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown role, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "admin"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/users/u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/users/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "reader"})
	s := newTestServer(t)
	s.db = newDB(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated Name", "email": "new@b.com", "role": "admin"})
	req := httptest.NewRequest("PUT", "/api/v1/users/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "Updated Name" {
		t.Errorf("expected Updated Name, got %v", resp["name"])
	}
}

func TestHandleUpdateUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/users/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "reader"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/users/u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/users/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Audit Handler Tests
// ---------------------------------------------------------------------------
