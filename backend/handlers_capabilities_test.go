package backend

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/backend/capability"
)

func TestHandleListCapabilities(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/projects/p1/capabilities", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleCreateCapability(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-capability", "description": "A test capability"})
	req := httptest.NewRequest("POST", "/api/v1/projects/p1/capabilities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-capability" {
		t.Errorf("expected test-capability, got %v", resp["name"])
	}
	if resp["name"] != "test-capability" {
		t.Errorf("expected test-capability, got %v", resp["name"])
	}
	if _, ok := resp["state"]; ok {
		t.Errorf("post-M0.8: state must not be returned on create (it is derived from Releases), got %v", resp["state"])
	}
}


func TestHandleGetCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Test", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleGetCapability_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/capabilities/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleUpdateCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Old", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newDB(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated", "description": "New desc"})
	req := httptest.NewRequest("PUT", "/api/v1/capabilities/c1", bytes.NewReader(body))
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


func TestHandleDeleteCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Test", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/capabilities/c1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}
