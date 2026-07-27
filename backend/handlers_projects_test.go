package backend

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/backend/capability"
)

func TestHandleListProjects(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces/w1/projects", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleCreateProject(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-project", "description": "A test"})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/w1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleCreateProject_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/w1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleGetProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Test", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/projects/p1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleGetProject_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleUpdateProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Old", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/projects/p1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleDeleteProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Test", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/p1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}
