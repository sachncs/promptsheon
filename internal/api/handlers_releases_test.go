package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/testdata"
)

func releaseTestManifest() capability.Manifest { return testdata.NewManifest() }

func seedReleaseFixture(repo *mockRepo) {
	repo.workspaces["w1"] = &capability.Workspace{ID: "w1", Name: "test"}
	repo.projects["p1"] = &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "test"}
	repo.capabilities["c1"] = &capability.Capability{ID: "c1", ProjectID: "p1", Name: "greeting"}
	repo.versions["v1"] = &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		Manifest: releaseTestManifest(), ManifestHash: "h1",
	}
}

func newReleaseTestServer(repo *mockRepo, svc *release.Service) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(newRepositories(repo), logger, WithReleaseService(svc))
}

func decodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// TestReleaseRoutesCreateVoteActivateInvoke exercises the
// production release lifecycle end-to-end. It wires a real
// invoke.Invoker (via newInvokeTestServer) so the /invoke
// step actually runs the request through the in-memory LLM
// provider and returns a real Execution record with token
// counts. The previous form of this test relied on a stub
// path that returned a fake row with tokens=0; that has
// been removed because it polluted the audit chain.
func TestReleaseRoutesCreateVoteActivateInvoke(t *testing.T) {
	repo := newMockRepo()
	seedReleaseFixture(repo)
	svc := release.NewService(repo, repo, approval.MakerCheckerPolicy{RequiredApprovers: 1})

	// Build a release-aware test server that uses the in-memory
	// provider. newInvokeTestServerWithRepo mounts a real
	// invoke.Invoker; WithReleaseService plugs in the release
	// service. We need a server with both.
	srv := newInvokeTestServerWithRepo(t, repo, WithReleaseService(svc))

	// 1. Create release (as "alice" per the fixture)
	body, _ := json.Marshal(map[string]string{"environment": "prod"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/versions/v1/releases", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.User{ID: "alice", Role: auth.RoleAdmin}))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create release: status=%d body=%s", w.Code, w.Body.String())
	}
	var rel release.Release
	decodeJSON(t, w.Body, &rel)
	if rel.Status != release.StatusPending {
		t.Fatalf("status = %q want pending", rel.Status)
	}

	// 2. Vote (as "bob" via the authenticated user context; the
	// request body's "identity" field is now ignored)
	body, _ = json.Marshal(map[string]string{"identity": "bob", "decision": "approve"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/votes", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.User{ID: "bob", Role: auth.RoleAdmin}))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("vote: status=%d body=%s", w.Code, w.Body.String())
	}

	// 3. Activate
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/activate", nil)
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("activate: status=%d body=%s", w.Code, w.Body.String())
	}
	decodeJSON(t, w.Body, &rel)
	if rel.Status != release.StatusActive {
		t.Fatalf("status = %q want active", rel.Status)
	}

	// 4. Invoke: hits the in-memory provider through the real
	// invoke.Invoker. Asserts the 201 + real Execution record.
	body, _ = json.Marshal(map[string]any{"inputs": map[string]any{"q": "hi"}})
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/invoke", bytes.NewReader(body))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("invoke: status=%d body=%s", w.Code, w.Body.String())
	}
	var exec capability.Execution
	decodeJSON(t, w.Body, &exec)
	if got, want := exec.TotalTokens, 2; got != want {
		t.Errorf("expected TotalTokens=%d, got %d", want, got)
	}
	if got, want := exec.CostUSD, 0.01; got != want {
		t.Errorf("expected CostUSD=%v, got %v", want, got)
	}

	// 5. Get approval trail
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/releases/"+rel.ID+"/approval", nil)
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get approval: status=%d body=%s", w.Code, w.Body.String())
	}
	var a approval.Approval
	decodeJSON(t, w.Body, &a)
	if len(a.Votes) != 1 || a.Votes[0].Identity != "bob" {
		t.Fatalf("votes = %+v want one from bob", a.Votes)
	}
}

func TestReleaseActivateQuorumConflict(t *testing.T) {
	repo := newMockRepo()
	seedReleaseFixture(repo)
	svc := release.NewService(repo, repo, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	srv := newReleaseTestServer(repo, svc)

	body, _ := json.Marshal(map[string]string{"environment": "prod"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/versions/v1/releases", bytes.NewReader(body))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status=%d", w.Code)
	}
	var rel release.Release
	decodeJSON(t, w.Body, &rel)

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/activate", nil)
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("activate without quorum: status=%d want 409, body=%s", w.Code, w.Body.String())
	}
}

type failingArtifactLoader struct{}

func (failingArtifactLoader) Load(context.Context, capability.ArtifactKind, string) ([]byte, error) {
	return nil, errors.New("CAS unavailable")
}

func TestReleaseInvokeResolverErrorReturns502(t *testing.T) {
	repo := newMockRepo()
	seedReleaseFixture(repo)
	svc := release.NewService(repo, repo, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	srv := newInvokeTestServerWithRepo(t, repo,
		WithReleaseService(svc),
		WithReleaseResolver(release.NewResolver(repo, failingArtifactLoader{})),
	)

	body, _ := json.Marshal(map[string]string{"environment": "prod"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/versions/v1/releases", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.User{ID: "alice", Role: auth.RoleAdmin}))
	srv.ServeHTTP(w, req)
	var rel release.Release
	decodeJSON(t, w.Body, &rel)

	body, _ = json.Marshal(map[string]string{"decision": "approve"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/releases/"+rel.ID+"/votes", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.User{ID: "bob", Role: auth.RoleAdmin}))
	srv.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/releases/"+rel.ID+"/activate", nil)
	srv.ServeHTTP(w, req)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/releases/"+rel.ID+"/invoke", bytes.NewReader([]byte(`{}`)))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway || !bytes.Contains(w.Body.Bytes(), []byte("CAS unavailable")) {
		t.Fatalf("invoke resolver failure: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReleaseInvokeNotActiveConflict(t *testing.T) {
	repo := newMockRepo()
	seedReleaseFixture(repo)
	svc := release.NewService(repo, repo, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	srv := newReleaseTestServer(repo, svc)

	body, _ := json.Marshal(map[string]string{"environment": "prod"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/versions/v1/releases", bytes.NewReader(body))
	srv.ServeHTTP(w, req)
	var rel release.Release
	decodeJSON(t, w.Body, &rel)

	body, _ = json.Marshal(map[string]any{"model": "claude"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/invoke", bytes.NewReader(body))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("invoke pending: status=%d want 409", w.Code)
	}
}
