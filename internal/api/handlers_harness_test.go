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
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/testdata"
)

func harnessRunnerWithStub(t *testing.T, repo *mockRepo) *harness.EvalRunner {
	t.Helper()
	return harness.NewEvalRunner(repo, &stubInvoker{out: json.RawMessage(`"hi"`)})
}

func TestHarnessDatasetCRUD(t *testing.T) {
	repo := newMockRepo()
	srv := newServerWithHarness(t, repo, harnessRunnerWithStub(t, repo))
	seedHarnessFixture(repo)

	createBody, _ := json.Marshal(map[string]any{
		"name": "greeting",
		"cases": []map[string]any{
			{"inputs": json.RawMessage(`"hi"`), "expected": json.RawMessage(`"hi"`)},
			{"inputs": json.RawMessage(`"bye"`), "expected": json.RawMessage(`"bye"`)},
		},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/capabilities/c1/datasets", bytes.NewReader(createBody))
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create dataset: %d body=%s", w.Code, w.Body.String())
	}
	ds := decodeHarnessResponse[harness.Dataset](t, w)
	if ds.ID == "" {
		t.Fatal("expected dataset id")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/capabilities/c1/datasets", nil)
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/datasets/"+ds.ID, nil)
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d", w.Code)
	}
	got := decodeHarnessResponse[harness.Dataset](t, w)
	if len(got.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(got.Cases))
	}

	runBody, _ := json.Marshal(map[string]any{
		"dataset_id": ds.ID,
		"scorer":     "exact_match",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/releases/r1/evals", bytes.NewReader(runBody))
	srv.ServeHTTP(w, req)
	// Handler returns 200 on RunPassed and 422 on RunFailed; accept
	// either since the stub only matches case 1 and we exercise the
	// failure path incidentally.
	if w.Code != http.StatusOK && w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("run eval: %d body=%s", w.Code, w.Body.String())
	}
	run := decodeHarnessResponse[harness.EvalRun](t, w)
	if run.Status != harness.RunPassed && run.Status != harness.RunFailed {
		t.Fatalf("unexpected status: %s", run.Status)
	}
	if run.Total != 2 {
		t.Fatalf("total = %d want 2", run.Total)
	}
	if run.Passed != 1 || run.Failed != 1 {
		t.Fatalf("counts: passed=%d failed=%d want 1/1", run.Passed, run.Failed)
	}
}

func TestHarnessPreconditionBlocksActivate(t *testing.T) {
	t.Setenv("PROMPTSHEON_HARNESS_PRECONDITIONS", "true")
	repo := newMockRepo()
	svc := release.NewService(repo, repo, approval.MakerCheckerPolicy{RequiredApprovers: 1}).
		WithHarness(harness.NewPreconditionRunner(), repo)
	_ = newReleaseTestServer(repo, svc)
	seedHarnessFixture(repo)

	if err := repo.CreatePrecondition(context.Background(), &harness.Precondition{
		ID: "p1", CapabilityID: "c1", Name: "fail", Command: "exit 5", TimeoutSec: 5, Enabled: true,
	}); err != nil {
		t.Fatalf("create prec: %v", err)
	}

	rel, err := svc.Create(context.Background(), "c1", 1, testdata.NewManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Vote(context.Background(), rel.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote: %v", err)
	}
	_, err = svc.Activate(context.Background(), rel.ID)
	if err == nil {
		t.Fatal("expected activate to fail when precondition fails")
	}
	if !errors.Is(err, harness.ErrPreconditionFailed) {
		t.Fatalf("expected ErrPreconditionFailed, got %v", err)
	}
}

func newServerWithHarness(t *testing.T, repo *mockRepo, runner *harness.EvalRunner) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(repo, logger,
		WithReleaseService(nil),
		WithHarnessRunner(runner),
	)
}

func decodeHarnessResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	return v
}

func seedHarnessFixture(repo *mockRepo) {
	repo.workspaces["w1"] = &capability.Workspace{ID: "w1", Name: "test"}
	repo.projects["p1"] = &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "test"}
	repo.capabilities["c1"] = &capability.Capability{ID: "c1", ProjectID: "p1", Name: "g"}
	repo.versions["v1"] = &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		Manifest: testdata.NewManifest(), ManifestHash: "h1",
	}
}

// stubInvoker implements harness.ReleaseInvoker so the API handler
// tests can run handleRunEval-style flows without a real LLM.
type stubInvoker struct {
	out json.RawMessage
	err error
}

func (s *stubInvoker) Invoke(_ context.Context, _ string, _ map[string]any) (json.RawMessage, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.out, nil
}
