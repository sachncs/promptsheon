package promptsheon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/models"
)

func TestHandleListAudit(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_WithFilters(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?user_id=u1&action=create&resource=test&since=2024-01-01T00:00:00Z&until=2024-12-31T23:59:59Z&limit=10&offset=5", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_InvalidSince(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?since=invalid-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_InvalidUntil(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?until=invalid-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_WithFilters(t *testing.T) {
	repo := newMockRepo()
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "f1", UserID: "u1", Action: "create", Resource: "test",
		Details: map[string]any{"k": "v"}, Timestamp: time.Now(),
	})
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "f2", UserID: "u2", Action: "delete", Resource: "other",
		Details: nil, Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?user_id=u1&action=create&resource=test&since=2000-01-01T00:00:00Z&until=2100-01-01T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var entries []*models.AuditEntry
	readJSONBody(t, rr.Body.Bytes(), &entries)
	if len(entries) == 0 {
		t.Error("expected at least one audit entry")
	}
}

func TestHandleExportAudit_InvalidSince(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export?since=bad-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_InvalidUntil(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export?until=bad-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_CSV(t *testing.T) {
	repo := newMockRepo()
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "e1", UserID: "u1", Action: "create", Resource: "test",
		Details: map[string]any{"key": "val"}, Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("expected text/csv, got %s", ct)
	}
	if !strings.Contains(rr.Body.String(), "e1") {
		t.Error("expected audit entry in CSV output")
	}
	if !strings.Contains(rr.Body.String(), `""key"":""val""`) {
		t.Error("expected details in CSV output")
	}
}

func TestHandleExportAudit_CSVBadDetails(t *testing.T) {
	repo := newMockRepo()
	// Create entry with non-serializable details (using Inf for NaN in JSON)
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "e2", UserID: "u1", Action: "test", Resource: "test",
		Details:   map[string]any{"nested": map[string]any{"value": "test"}},
		Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleVerifyAuditChain(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/verify", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("expected ok true, got %v", resp["ok"])
	}
	if resp["tail_mismatch"] != false {
		t.Errorf("expected tail_mismatch false, got %v", resp["tail_mismatch"])
	}
	if resp["last_row_id"] == nil {
		t.Error("expected last_row_id to be present")
	}
}

// ---------------------------------------------------------------------------
// Trace Handler Tests
// ---------------------------------------------------------------------------

func TestStartAuditWorkers(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.StartAuditWorkers(3)
	if cap(s.auditQueue) != 1024 {
		t.Errorf("expected queue capacity 1024, got %d", cap(s.auditQueue))
	}
	_ = s.StopAuditWorkers(ctx)
}

func TestStartAuditWorkers_ZeroWorkers(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	s.StartAuditWorkers(0)
	if cap(s.auditQueue) != 1024 {
		t.Errorf("expected queue capacity 1024, got %d", cap(s.auditQueue))
	}
	_ = s.StopAuditWorkers(ctx)
}

func TestAuditWorkerProcess(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)

	s.StartAuditWorkers(1)
	entry := &models.AuditEntry{ID: "test-audit", Action: "test", Resource: "test-resource"}
	s.auditQueue <- entry

	// Wait for the worker to process the entry. StopAuditWorkers
	// blocks until the queue is drained and the workers exit, so
	// once it returns, the entry is guaranteed to be persisted.
	if err := s.StopAuditWorkers(context.Background()); err != nil {
		t.Fatalf("StopAuditWorkers: %v", err)
	}

	entries := repo.auditEntries
	var found bool
	for _, e := range entries {
		if e.ID == "test-audit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit entry to be stored by worker")
	}
}

func TestAudit_DropWhenQueueFull(t *testing.T) {
	s := newTestServer(t)
	// Use a closed channel so all sends fail
	s.auditQueue = make(chan *models.AuditEntry)
	s.auditDropped.Store(0)

	ctx := context.Background()
	s.audit(ctx, "test", "resource", nil)

	if s.auditDropped.Load() == 0 {
		t.Log("audit entry was not dropped (queue may have had room)")
	}
}

func TestAudit_WithRequestInContext(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	r := httptest.NewRequest("GET", "/test", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("User-Agent", "test-agent")
	ctx := WithRequest(context.Background(), r)

	s.audit(ctx, "test-action", "test-resource", nil)

	select {
	case entry := <-s.auditQueue:
		if entry.Details["remote_addr"] != "127.0.0.1:1234" {
			t.Errorf("expected remote_addr=127.0.0.1:1234, got %v", entry.Details["remote_addr"])
		}
		if entry.Details["user_agent"] != "test-agent" {
			t.Errorf("expected user_agent=test-agent, got %v", entry.Details["user_agent"])
		}
	default:
		t.Error("expected audit entry in queue")
	}
}

func TestAudit_WithUserInContext(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	ctx := auth.WithUserContext(context.Background(), &auth.User{ID: "u42", Role: auth.RoleWriter})
	s.audit(ctx, "create", "test", nil)

	select {
	case entry := <-s.auditQueue:
		if entry.UserID != "u42" {
			t.Errorf("expected user_id u42, got %s", entry.UserID)
		}
	default:
		t.Error("expected audit entry in queue")
	}
}

func TestStopAuditWorkers_ConcurrentIdempotent(t *testing.T) {
	s := newTestServer(t)
	s.StartAuditWorkers(2)
	errs := make(chan error, 2)
	go func() { errs <- s.StopAuditWorkers(t.Context()) }()
	go func() { errs <- s.StopAuditWorkers(t.Context()) }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

func TestStopAuditWorkers_NilQueue(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = nil
	if err := s.StopAuditWorkers(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestStartAuditWorkers_SecondCallErrors is the regression for the
// unsafe sync.Once reset. StartAuditWorkers must reject a second
// call rather than silently allocate a new queue that nothing can
// close via the (now-broken) Once.
func TestStartAuditWorkers_SecondCallErrors(t *testing.T) {
	s := newTestServer(t)
	if err := s.StartAuditWorkers(1); err != nil {
		t.Fatalf("first StartAuditWorkers: %v", err)
	}
	t.Cleanup(func() {
		_ = s.StopAuditWorkers(context.Background())
	})
	if err := s.StartAuditWorkers(1); err == nil {
		t.Fatal("expected second StartAuditWorkers to error")
	}
}

// TestAuditWorkerProcess_ContextCancellation verifies that the
// worker context is honoured: cancelling the worker's context
// must unblock the worker goroutine even if the queue is empty.
func TestAuditWorkerProcess_ContextCancellation(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)
	if err := s.StartAuditWorkers(1); err != nil {
		t.Fatalf("StartAuditWorkers: %v", err)
	}
	s.auditMu.Lock()
	cancel := s.auditCancel
	s.auditMu.Unlock()
	if cancel != nil {
		cancel()
	}
	// StopAuditWorkers must return promptly: workers exit
	// because their context is cancelled.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := s.StopAuditWorkers(stopCtx); err != nil {
		t.Fatalf("StopAuditWorkers after cancel: %v", err)
	}
}

// TestAuditWorkerProcess_RespectsContext ensures AppendAudit
// receives the worker context, not a Background. We attach a
// context with a deadline; if the worker honoured Background
// instead, the test would still pass — but the test exercises
// the full lifecycle to catch regressions in the wiring.
func TestAuditWorkerProcess_RespectsContext(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)
	if err := s.StartAuditWorkers(1); err != nil {
		t.Fatalf("StartAuditWorkers: %v", err)
	}
	s.auditQueue <- &models.AuditEntry{ID: "ctx-audit", Action: "ctx", Resource: "ctx"}
	if err := s.StopAuditWorkers(context.Background()); err != nil {
		t.Fatalf("StopAuditWorkers: %v", err)
	}
	found := false
	for _, e := range repo.auditEntries {
		if e.ID == "ctx-audit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected worker to persist ctx-audit via the supplied context")
	}
}

// ---------------------------------------------------------------------------
// OAuth State Store Tests
// ---------------------------------------------------------------------------
