package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/backend/capability"
)

func TestHandleCreateVersionWithManifest(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c-manifest", Name: "manifest-test", ProjectID: "p1"})
	capID := "c-manifest"

	good := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	body, err := json.Marshal(map[string]any{"version": 1, "manifest": good})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/v1/capabilities/"+capID+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid manifest, got %d: %s", rr.Code, rr.Body.String())
	}
	var got capability.Version
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ManifestHash == "" {
		t.Fatalf("expected manifest_hash to be set, got empty")
	}

	body, _ = json.Marshal(map[string]any{"version": 2, "manifest": capability.Manifest{
		Prompt: capability.ArtifactRef{Kind: capability.ArtifactKind("bogus"), Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}})
	req = httptest.NewRequest("POST", "/api/v1/capabilities/"+capID+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid manifest kind, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Vault Handler Tests
// ---------------------------------------------------------------------------


func TestHandleListVersions(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1/versions", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleCreateVersion(t *testing.T) {
	s := newTestServer(t)
	good := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	body, _ := json.Marshal(map[string]any{"version": 1, "manifest": good})
	req := httptest.NewRequest("POST", "/api/v1/capabilities/c1/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["version"] != float64(1) {
		t.Errorf("expected version 1, got %v", resp["version"])
	}
}


func TestHandleGetVersion(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v1", Version: 1, CapabilityID: "c1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/versions/v1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}


func TestHandleGetLatestVersion(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v1", Version: 1, CapabilityID: "c1"})
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v2", Version: 2, CapabilityID: "c1"})
	s := newTestServerWithRepo(t, repo)

	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1/versions/latest", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["id"] != "v2" {
		t.Errorf("expected v2, got %v", resp["id"])
	}
}
