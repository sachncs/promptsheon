package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestHandleListWorkspaces(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWorkspace(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-workspace", "organization": "TestOrg"})
	req := httptest.NewRequest("POST", "/api/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-workspace" {
		t.Errorf("expected test-workspace, got %v", resp["name"])
	}
}

func TestHandleCreateWorkspace_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{})
	req := httptest.NewRequest("POST", "/api/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Test"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/workspaces/w1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetWorkspace_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Old"})
	s := newTestServer(t)
	s.db = newDB(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/workspaces/w1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "Updated" {
		t.Errorf("expected Updated, got %v", resp["name"])
	}
}

func TestHandleDeleteWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Test"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/workspaces/w1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}
