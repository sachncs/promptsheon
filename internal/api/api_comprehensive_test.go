package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"promptsheon/internal/alerting"
	"promptsheon/internal/guardrail"
	"promptsheon/internal/metrics"
	"promptsheon/internal/models"
	"promptsheon/internal/snapshot"
	"promptsheon/internal/store"
	"promptsheon/internal/trace"
	"promptsheon/internal/vault"
	"promptsheon/internal/webhook"

	contextpkg "promptsheon/internal/context"
)

func setupTestServerWithDeps(t *testing.T) (*Server, *store.SQLite) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	traceDBPath := filepath.Join(dir, "traces.db")
	traceDir := filepath.Dir(traceDBPath)
	_ = os.MkdirAll(traceDir, 0o755)
	traceDB, err := store.NewSQLite(traceDBPath)
	if err != nil {
		t.Fatalf("NewSQLite trace: %v", err)
	}
	spans, err := trace.NewSQLite(traceDB.DB())
	if err != nil {
		t.Fatalf("NewSQLite trace: %v", err)
	}

	collector := metrics.NewCollector()

	ss, _ := snapshot.NewStore(db.DB())

	vaultKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := vault.New(vaultKey)
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}

	wDisp := webhook.NewDispatcher(nil)

	gMgr := guardrail.NewManager(logger, collector)
	aMgr := alerting.NewManager(logger, collector)
	cMgr := contextpkg.NewManager(db)

	usageTracker := NewUsageTracker()

	srv := NewServer(db, logger,
		WithTracing(spans, collector),
		WithSnapshotStore(ss),
		WithWebhooks(wDisp),
		WithVault(v),
		WithUsageTracker(usageTracker),
		WithGuardrailManager(gMgr),
		WithAlertingManager(aMgr),
		WithContextManager(cMgr),
		WithServerConfig(&ServerConfig{
			CircuitBreakerFailureThreshold: 5,
			CircuitBreakerSuccessThreshold: 3,
			CircuitBreakerCooldown:         30,
		}),
	)

	t.Cleanup(func() {
		spans = nil
		traceDB.Close()
	})

	return srv, db
}

func setupTestServerMinimal(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(db, logger)
	return srv
}

func doReq(t *testing.T, method, url string, body string) *http.Response {
	t.Helper()
	var resp *http.Response
	var err error
	switch method {
	case "GET":
		resp, err = http.Get(url)
	case "POST":
		resp, err = http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	case "PUT":
		req, _ := http.NewRequest("PUT", url, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
	case "DELETE":
		req, _ := http.NewRequest("DELETE", url, nil)
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// =============================================================================
// Health Tests
// =============================================================================

func TestHealthEndpointComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", body["status"])
	}
}

func TestReadyEndpointComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Prompt Tests
// =============================================================================

func TestPromptCreateAndGetComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"test-prompt","content":"You are helpful.","tags":["test"],"description":"A test prompt","model_hint":"gpt-4","environment":"prod","metadata":{"key":"val"}}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created models.Prompt
	json.NewDecoder(resp.Body).Decode(&created)
	if created.Name != "test-prompt" {
		t.Fatalf("expected name, got %q", created.Name)
	}
	if created.Version != 1 {
		t.Fatalf("expected version 1, got %d", created.Version)
	}
	if created.Status != models.StatusDraft {
		t.Fatalf("expected draft, got %q", created.Status)
	}

	resp, err = http.Get(ts.URL + "/api/v1/prompts/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var prompts []models.Prompt
	json.NewDecoder(resp.Body).Decode(&prompts)
	if len(prompts) != 1 {
		t.Fatalf("expected 1, got %d", len(prompts))
	}
}

func TestPromptUpdateComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(`{"name":"test","content":"hello"}`)))
	var created models.Prompt
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	updateBody := `{"name":"updated","description":"new desc","tags":["a"],"metadata":{"k":"v"}}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+created.ID, bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updateBody2 := `{"content":"new content","variables":[{"name":"x","type":"string","required":true}],"model_hint":"gpt-3.5"}`
	req2, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+created.ID, bytes.NewReader([]byte(updateBody2)))
	req2.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("PUT content: %v", err)
	}
	defer resp.Body.Close()
	var updated models.Prompt
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Version != 2 {
		t.Fatalf("expected version 2, got %d", updated.Version)
	}
}

func TestPromptDeployAndArchiveComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	p := &models.Prompt{ID: "dp-1", Name: "deploy-test", Content: "hi", Status: models.StatusApproved, CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p)

	resp, err := http.Post(ts.URL+"/api/v1/prompts/dp-1/deploy", "", nil)
	if err != nil {
		t.Fatalf("POST deploy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var deployed models.Prompt
	json.NewDecoder(resp.Body).Decode(&deployed)
	if deployed.Status != models.StatusDeployed {
		t.Fatalf("expected deployed, got %q", deployed.Status)
	}

	resp, err = http.Post(ts.URL+"/api/v1/prompts/dp-1/archive", "", nil)
	if err != nil {
		t.Fatalf("POST archive: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPromptDeployNotApprovedComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	p := &models.Prompt{ID: "dp-2", Name: "draft", Content: "hi", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p)

	resp, err := http.Post(ts.URL+"/api/v1/prompts/dp-2/deploy", "", nil)
	if err != nil {
		t.Fatalf("POST deploy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptArchiveWrongStatusComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	p := &models.Prompt{ID: "arc-1", Name: "draft", Content: "hi", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p)

	resp, err := http.Post(ts.URL+"/api/v1/prompts/arc-1/archive", "", nil)
	if err != nil {
		t.Fatalf("POST archive: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/prompts/nonexistent", ""},
		{"DELETE", "/api/v1/prompts/nonexistent", ""},
		{"POST", "/api/v1/prompts/nonexistent/deploy", "{}"},
		{"POST", "/api/v1/prompts/nonexistent/archive", "{}"},
		{"POST", "/api/v1/prompts/nonexistent/run", `{"variables":{}}`},
		{"POST", "/api/v1/prompts/nonexistent/preview", `{"variables":{}}`},
		{"POST", "/api/v1/prompts/nonexistent/stream", `{"variables":{}}`},
		{"GET", "/api/v1/prompts/nonexistent/versions", ""},
		{"POST", "/api/v1/prompts/nonexistent/restore", `{"cas_hash":"abc"}`},
	}
	for _, tt := range tests {
		resp := doReq(t, tt.method, ts.URL+tt.path, tt.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestPromptValidationErrorsComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/prompts", `{invalid`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/prompts", `{"content":"hi"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/prompts", `{"name":"test"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing content, got %d", resp.StatusCode)
	}
}

func TestPromptUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/prompts/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPromptUpdateInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "upd-bad", Name: "t", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/prompts/upd-bad", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptPreviewComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{
		ID: "pv-1", Name: "preview", Content: "Hello {{name}}, welcome to {{place}}.",
		Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now,
		Binding: &models.ProviderBinding{Provider: "openai", Model: "gpt-4"},
	})

	resp := doReq(t, "POST", ts.URL+"/api/v1/prompts/pv-1/preview", `{"variables":{"name":"Alice","place":"Wonderland"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["rendered"] != "Hello Alice, welcome to Wonderland." {
		t.Fatalf("unexpected rendered: %v", result["rendered"])
	}
}

func TestFindSimilarPromptsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "sim-1", Name: "a", Content: "You are a helpful assistant.", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})
	db.CreatePrompt(ctx, &models.Prompt{ID: "sim-2", Name: "b", Content: "Be a helpful and friendly assistant.", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/prompts/similar?content=helpful+assistant&threshold=0.5")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFindSimilarPromptsMissingContentComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/prompts/similar")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptVersionsEmptyCASComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "ver-1", Name: "ver", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/prompts/ver-1/versions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entries []any
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 0 {
		t.Fatalf("expected 0, got %d", len(entries))
	}
}

func TestPromptRestoreMissingHashComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "rst-1", Name: "rst", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/prompts/rst-1/restore", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptListWithSearchComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "srch-1", Name: "alpha", Content: "hi", Environment: "prod", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/prompts?search=alpha&environment=prod")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var prompts []models.Prompt
	json.NewDecoder(resp.Body).Decode(&prompts)
	if len(prompts) != 1 {
		t.Fatalf("expected 1, got %d", len(prompts))
	}
}

// =============================================================================
// Agent Tests
// =============================================================================

func TestAgentCRUDComprehensive2(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"research-agent","description":"Multi-step agent","steps":[{"id":"s1","output_key":"out1"}],"tools":[{"name":"http","type":"http","config":{}}]}`
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created models.Agent
	json.NewDecoder(resp.Body).Decode(&created)

	resp, err = http.Get(ts.URL + "/api/v1/agents/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/agents")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var agents []models.Agent
	json.NewDecoder(resp.Body).Decode(&agents)
	if len(agents) != 1 {
		t.Fatalf("expected 1, got %d", len(agents))
	}

	updateBody := `{"name":"updated-agent","description":"Updated"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/agents/"+created.ID, bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/agents/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestAgentNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/agents/nonexistent", ""},
		{"DELETE", "/api/v1/agents/nonexistent", ""},
		{"GET", "/api/v1/agents/nonexistent/export", ""},
		{"POST", "/api/v1/agents/nonexistent/fork", "{}"},
		{"GET", "/api/v1/agents/nonexistent/versions", ""},
		{"POST", "/api/v1/agents/nonexistent/restore", `{"version":1}`},
		{"POST", "/api/v1/agents/nonexistent/deploy", "{}"},
		{"POST", "/api/v1/agents/nonexistent/archive", "{}"},
		{"POST", "/api/v1/agents/nonexistent/rerun", `{"input":{}}`},
		{"POST", "/api/v1/agents/nonexistent/execute", `{"input":{}}`},
		{"POST", "/api/v1/agents/nonexistent/guardrail-config", `{"name":"x"}`},
		{"GET", "/api/v1/agents/nonexistent/guardrail-config", ""},
		{"GET", "/api/v1/agents/nonexistent/executions", ""},
	}
	for _, tt := range tests {
		resp := doReq(t, tt.method, ts.URL+tt.path, tt.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestAgentCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents", `{"description":"test"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/agents", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentForkComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{
		ID: "fork-orig", Name: "original", Description: "orig",
		Steps: []models.AgentStep{{ID: "s1", OutputKey: "out1"}},
		Tools: []models.ToolRef{{Name: "http", Type: models.ToolHTTP, Config: map[string]any{"url": "http://example.com"}}},
		CreatedBy: "u", CreatedAt: now, UpdatedAt: now,
	})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/fork-orig/fork", `{"name":"my-fork"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var forked models.Agent
	json.NewDecoder(resp.Body).Decode(&forked)
	if forked.Name != "my-fork" {
		t.Fatalf("expected my-fork, got %q", forked.Name)
	}
	if forked.ParentID != "fork-orig" {
		t.Fatalf("expected parent_id, got %q", forked.ParentID)
	}

	resp2 := doReq(t, "POST", ts.URL+"/api/v1/agents/fork-orig/fork", `{}`)
	defer resp2.Body.Close()
	var forked2 models.Agent
	json.NewDecoder(resp2.Body).Decode(&forked2)
	if forked2.Name != "original (fork)" {
		t.Fatalf("expected default name, got %q", forked2.Name)
	}
}

func TestAgentExportJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "exp-1", Name: "export-me", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/agents/exp-1/export")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentExportYAMLComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "exp-yaml", Name: "yaml-agent", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/agents/exp-yaml/export?format=yaml")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-yaml" {
		t.Fatalf("expected yaml, got %q", ct)
	}
}

func TestAgentImportYAMLComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	yamlBody := "name: imported-agent\ndescription: imported\nsteps:\n  - id: step1\n    output_key: out1\n"
	resp, err := http.Post(ts.URL+"/api/v1/agents/import-yaml", "application/x-yaml", bytes.NewReader([]byte(yamlBody)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestAgentImportYAMLInvalidComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/agents/import-yaml", "text/plain", bytes.NewReader([]byte(`{{{invalid yaml`)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentVersionsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "av-1", Name: "versioned", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/agents/av-1/versions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentDeployComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "ad-1", Name: "deployable", Status: models.StatusApproved, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/ad-1/deploy", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentDeployNotApprovedComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "ad-2", Name: "draft-agent", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/ad-2/deploy", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentArchiveComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aar-1", Name: "archivable", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/aar-1/archive", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentRerunComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "arr-1", Name: "rerunnable", Status: models.StatusDraft, Steps: []models.AgentStep{{ID: "s1", OutputKey: "o1"}}, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/arr-1/rerun", `{"input":{"key":"val"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentExecuteComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aex-1", Name: "executable", Status: models.StatusApproved, Steps: []models.AgentStep{{ID: "s1", OutputKey: "o1"}}, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/aex-1/execute", `{"input":{"q":"hi"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentExecuteNotApprovedComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aex-2", Name: "draft", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/aex-2/execute", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentExecutionsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aex-l", Name: "exec-list", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp, err := http.Get(ts.URL + "/api/v1/agents/aex-l/executions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentExecutionGetNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/agents/fake/executions/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAgentGuardrailConfigCRUDComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "agc-1", Name: "guardrailed", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	body := `{"name":"cost-guard","enabled":true,"max_cost_per_run":5.0,"max_latency_ms":30000,"max_tokens_per_step":4096,"content_policy":["no_pii"],"restricted_terms":["secret"],"stop_on_violation":true}`
	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/agc-1/guardrail-config", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var cfg models.AgentGuardrailConfig
	json.NewDecoder(resp.Body).Decode(&cfg)

	resp = doReq(t, "GET", ts.URL+"/api/v1/agents/agc-1/guardrail-config", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/agents/agc-1/guardrail-config/"+cfg.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentGuardrailConfigNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "GET", ts.URL+"/api/v1/agents/fake/guardrail-config", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/agents/fake/guardrail-config", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAgentUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/agents/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAgentUpdateInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "auj-1", Name: "t", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/agents/auj-1", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentUpdateStatusComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aus-1", Name: "status-test", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	// Create a review and approve it first
	revResp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"aus-1","resource_type":"agent","author":"reviewer"}`)
	defer revResp.Body.Close()
	var rev models.Review
	json.NewDecoder(revResp.Body).Decode(&rev)

	approveResp := doReq(t, "PUT", ts.URL+"/api/v1/reviews/"+rev.ID+"/approve", `{"comment":"looks good"}`)
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve review expected 200, got %d", approveResp.StatusCode)
	}

	resp := doReq(t, "PUT", ts.URL+"/api/v1/agents/aus-1", `{"status":"approved"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentUpdateStatusCannotApproveWithoutReviewComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "aus-2", Name: "no-review", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/agents/aus-2", `{"status":"approved"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentCreateDAGValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"circular","steps":[{"id":"s1","depends_on":["s2"],"output_key":"o1"},{"id":"s2","depends_on":["s1"],"output_key":"o2"}]}`
	resp := doReq(t, "POST", ts.URL+"/api/v1/agents", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for circular, got %d", resp.StatusCode)
	}
}

func TestListTemplatesComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/agents/templates")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var templates []*models.Agent
	json.NewDecoder(resp.Body).Decode(&templates)
	if templates == nil {
		templates = []*models.Agent{}
	}
	if len(templates) != 0 {
		t.Fatalf("expected 0, got %d", len(templates))
	}
}

func TestAgentValidateWorkflowComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/validate", `{"steps":[{"id":"s1","output_key":"o1"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
}

func TestAgentRerunNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/nonexistent/rerun", `{"input":{}}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAgentRestoreNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/agents/nonexistent/restore", `{"version":1}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Context Tests
// =============================================================================

func TestContextCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"test-ctx","description":"test context","type":"conversation","system_prompt":"You are helpful.","messages":[{"role":"user","content":"hi"}],"token_budget":4096,"truncation_strategy":"sliding_window","metadata":{"key":"val"}}`
	resp := doReq(t, "POST", ts.URL+"/api/v1/contexts", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created models.Context
	json.NewDecoder(resp.Body).Decode(&created)

	resp = doReq(t, "GET", ts.URL+"/api/v1/contexts/"+created.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/contexts", "")
	defer resp.Body.Close()
	var contexts []models.Context
	json.NewDecoder(resp.Body).Decode(&contexts)
	if len(contexts) != 1 {
		t.Fatalf("expected 1, got %d", len(contexts))
	}

	updateBody := `{"name":"updated-ctx","description":"updated","system_prompt":"new prompt","token_budget":8192,"truncation_strategy":"drop_oldest","metadata":{"k2":"v2"}}`
	resp = doReq(t, "PUT", ts.URL+"/api/v1/contexts/"+created.ID, updateBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/contexts/"+created.ID+"/messages", `{"role":"assistant","content":"Hello!"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/contexts/"+created.ID+"/messages", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/contexts/"+created.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContextNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/contexts/nonexistent", ""},
		{"DELETE", "/api/v1/contexts/nonexistent", ""},
		{"POST", "/api/v1/contexts/nonexistent/messages", `{"role":"user","content":"hi"}`},
		{"DELETE", "/api/v1/contexts/nonexistent/messages", ""},
		{"POST", "/api/v1/contexts/nonexistent/assemble", `{"variables":{}}`},
	}
	for _, tt := range tests {
		resp := doReq(t, tt.method, ts.URL+tt.path, tt.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestContextCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/contexts", `{"description":"test"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/contexts", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestContextAppendMessageValidationComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateContext(ctx, &models.Context{ID: "ctx-msg", Name: "t", CreatedAt: now, UpdatedAt: now, Messages: []models.ContextMessage{}})

	resp := doReq(t, "POST", ts.URL+"/api/v1/contexts/ctx-msg/messages", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/contexts/ctx-msg/messages", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestContextAssembleNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/contexts/nonexistent/assemble", `{"variables":{}}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestContextUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/contexts/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestContextAssembleWithVariablesComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateContext(ctx, &models.Context{
		ID: "ctx-asm", Name: "asm", Type: models.ContextSystemPrompt,
		SystemPrompt: "You are {{role}}.", TokenBudget: 4096,
		TruncationStrategy: models.TruncationSlidingWindow,
		Messages:           []models.ContextMessage{},
		CreatedAt: now, UpdatedAt: now,
	})

	resp := doReq(t, "POST", ts.URL+"/api/v1/contexts/ctx-asm/assemble", `{"variables":{"role":"assistant"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Guardrails Tests
// =============================================================================

func TestGuardrailRulesCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/guardrails/rules")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := `{"name":"no-pii","type":"content_policy","severity":"high","config":{"patterns":["SSN"]},"environments":["prod"],"prompt_ids":["p1"],"agent_ids":["a1"]}`
	resp = doReq(t, "POST", ts.URL+"/api/v1/guardrails/rules", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var rule guardrail.Rule
	json.NewDecoder(resp.Body).Decode(&rule)

	resp = doReq(t, "GET", ts.URL+"/api/v1/guardrails/rules/"+rule.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/guardrails/rules/nonexistent", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	updateBody := `{"name":"updated-rule","enabled":false,"config":{"patterns":["updated"]},"environments":["staging"],"prompt_ids":["p2"],"agent_ids":["a2"]}`
	resp = doReq(t, "PUT", ts.URL+"/api/v1/guardrails/rules/"+rule.ID, updateBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/guardrails/rules/"+rule.ID, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestGuardrailCheckComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/guardrails/check", `{"content":"hello world","model":"gpt-4","environment":"prod"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["passed"] != true {
		t.Fatalf("expected passed=true, got %v", result["passed"])
	}
}

func TestGuardrailCheckInvalidJSONComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/guardrails/check", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGuardrailViolationsComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/guardrails/violations")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGuardrailResolveViolationNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/guardrails/violations/nonexistent/resolve", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGuardrailCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/guardrails/rules", `{"type":"content_policy"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/guardrails/rules", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGuardrailUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/guardrails/rules/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGuardrailNoManagerComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "GET", ts.URL+"/api/v1/guardrails/rules", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/guardrails/rules", `{"name":"x","type":"y"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/guardrails/rules/123", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/guardrails/check", `{"content":"hi"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/guardrails/violations", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Alerting Tests
// =============================================================================

func TestAlertingRulesCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/alerts/rules")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := `{"name":"high-latency","type":"latency","severity":"high","threshold":5000,"duration_minutes":5,"window_minutes":15,"config":{"model":"gpt-4"}}`
	resp = doReq(t, "POST", ts.URL+"/api/v1/alerts/rules", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var rule alerting.AlertRule
	json.NewDecoder(resp.Body).Decode(&rule)

	resp = doReq(t, "GET", ts.URL+"/api/v1/alerts/rules/"+rule.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/alerts/rules/nonexistent", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	updateBody := `{"name":"updated-alert","enabled":false,"threshold":10000,"config":{"updated":true}}`
	resp = doReq(t, "PUT", ts.URL+"/api/v1/alerts/rules/"+rule.ID, updateBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/alerts/rules/"+rule.ID, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestAlertingAlertsAndNotificationsComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/alerts/active")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "PUT", ts.URL+"/api/v1/alerts/active/nonexistent/resolve", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/alerts/notifications", `{"name":"oncall","channels":["slack","email"]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestAlertingCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/alerts/rules", `{"type":"latency"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/alerts/rules", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAlertingUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/alerts/rules/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAlertingNoManagerComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "GET", ts.URL+"/api/v1/alerts/rules", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/alerts/rules", `{"name":"x","type":"y"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/alerts/rules/123", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "GET", ts.URL+"/api/v1/alerts/active", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "PUT", ts.URL+"/api/v1/alerts/active/x/resolve", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/alerts/notifications", `{"name":"x","channels":["slack"]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Webhooks Tests
// =============================================================================

func TestWebhooksCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/webhooks")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := `{"url":"https://example.com/hook","events":["prompt.deployed","review.approved"],"secret":"s3cret"}`
	resp = doReq(t, "POST", ts.URL+"/api/v1/webhooks", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/webhooks", `{"events":["prompt.deployed"]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/webhooks", `{"url":"https://example.com"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/webhooks", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhookDeleteComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/webhooks", `{"url":"https://example.com/hook","events":["prompt.deployed"]}`)
	var wh struct {
		ID string
	}
	json.NewDecoder(resp.Body).Decode(&wh)
	resp.Body.Close()

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/webhooks/"+wh.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWebhooksNoDispatcherComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "GET", ts.URL+"/api/v1/webhooks", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/webhooks", `{"url":"x","events":["a"]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestWebhookDeleteNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "DELETE", ts.URL+"/api/v1/webhooks/nonexistent", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Workflow Tests
// =============================================================================

func TestWorkflowRunComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "wf-ag", Name: "wf-agent", Status: models.StatusDraft, Steps: []models.AgentStep{{ID: "s1", OutputKey: "o1"}}, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/workflows/run", `{"agent_id":"wf-ag","input":{"key":"val"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/workflows/run", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/workflows/run", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/workflows/run", `{"agent_id":"nonexistent"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	resp, err := http.Get(ts.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/workflows?agent_id=wf-ag&status=completed")
	if err != nil {
		t.Fatalf("GET filtered: %v", err)
	}
	defer resp.Body.Close()
}

func TestWorkflowGetNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/workflows/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/workflows/nonexistent/steps")
	if err != nil {
		t.Fatalf("GET steps: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWorkflowCancelComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.SaveWorkflow(ctx, &models.Workflow{
		ID: "wf-c", AgentID: "a1", Status: models.WorkflowRunning,
		StartedAt: now, CreatedAt: now,
	})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/workflows/wf-c/cancel", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWorkflowCancelNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/workflows/nonexistent/cancel", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWorkflowCancelConflictComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.SaveWorkflow(ctx, &models.Workflow{
		ID: "wf-conf", AgentID: "a1", Status: models.WorkflowCompleted,
		StartedAt: now, CreatedAt: now,
	})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/workflows/wf-conf/cancel", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestWorkflowStepsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.SaveWorkflow(ctx, &models.Workflow{
		ID: "wf-1", AgentID: "a1", Status: models.WorkflowCompleted,
		Input: map[string]any{"k": "v"}, StartedAt: now, CreatedAt: now,
	})

	resp, err := http.Get(ts.URL + "/api/v1/workflows/wf-1/steps")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Metrics Tests
// =============================================================================

func TestMetricsSummaryComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMetricsTopPromptsComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics/top-prompts")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMetricsTopAgentsComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics/top-agents")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMetricsDashboardComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics/dashboard")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUsageTrackerComprehensive(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordPromptUsage("p1", "prompt1", 100, 50.0)
	ut.RecordPromptUsage("p1", "prompt1", 200, 75.0)
	ut.RecordAgentUsage("a1", "agent1")

	top := ut.GetTopPrompts(10)
	if len(top) != 1 {
		t.Fatalf("expected 1, got %d", len(top))
	}
	if top[0].Count != 2 {
		t.Fatalf("expected count 2, got %d", top[0].Count)
	}

	topA := ut.GetTopAgents(10)
	if len(topA) != 1 {
		t.Fatalf("expected 1, got %d", len(topA))
	}

	ut.RecordPromptUsage("p2", "prompt2", 50, 25.0)
	top = ut.GetTopPrompts(1)
	if len(top) != 1 {
		t.Fatalf("expected limit 1, got %d", len(top))
	}
}

func TestMetricsPrometheusComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMetricsPrometheusNoCollectorComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/metrics")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSearchSpansComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/logs/search?operation=prompt.run")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Vault Tests
// =============================================================================

func TestVaultCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/vault/keys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := `{"provider_name":"openai","key_name":"main","key":"sk-test123"}`
	resp = doReq(t, "POST", ts.URL+"/api/v1/vault/keys", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/vault/keys", `{"provider_name":"openai"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/vault/keys", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/vault/keys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var keys []map[string]any
	json.NewDecoder(resp.Body).Decode(&keys)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}

func TestVaultDeleteNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "DELETE", ts.URL+"/api/v1/vault/keys/nonexistent", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestVaultNoVaultComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/vault/keys", `{"provider_name":"x","key_name":"y","key":"z"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Traces Tests
// =============================================================================

func TestTracesListAndGetComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/traces")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/traces?operation=prompt.run&service=promptsheon")
	if err != nil {
		t.Fatalf("GET filtered: %v", err)
	}
	defer resp.Body.Close()

	resp, err = http.Get(ts.URL + "/api/v1/traces/nonexistent")
	if err != nil {
		t.Fatalf("GET span: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/traces/tree/nonexistent")
	if err != nil {
		t.Fatalf("GET tree: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with empty list, got %d", resp.StatusCode)
	}
}

func TestTracesListWithTimeFilterComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	until := time.Now().Format(time.RFC3339)
	resp, err := http.Get(fmt.Sprintf(ts.URL+"/api/v1/traces?since=%s&until=%s", since, until))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Snapshots Tests
// =============================================================================

func TestSnapshotsListComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/snapshots")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/snapshots?prompt_hash=abc&model=gpt-4")
	if err != nil {
		t.Fatalf("GET filtered: %v", err)
	}
	defer resp.Body.Close()
}

func TestSnapshotGetNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/snapshots/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSnapshotsNoStoreComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/snapshots")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/snapshots/123")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Providers Tests
// =============================================================================

func TestProvidersListComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/providers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProvidersGetNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/providers/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestProviderTestUnavailableComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/providers/nonexistent/test", `{"model":"gpt-4"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestProviderTestInvalidJSONComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/providers/openai/test", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Auth Tests
// =============================================================================

func TestAPIKeyCRUDComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"test-key","user_id":"u1","role":"reader"}`
	resp := doReq(t, "POST", ts.URL+"/api/v1/apikeys", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var keyResp map[string]any
	json.NewDecoder(resp.Body).Decode(&keyResp)
	if keyResp["key"] == nil || keyResp["key"] == "" {
		t.Fatal("expected non-empty key")
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/apikeys", `{"name":"k"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/apikeys", `{"name":"k","user_id":"u","role":"superadmin"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/apikeys", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp, err := http.Get(ts.URL + "/api/v1/apikeys?user_id=u1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/apikeys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAPIKeyRevokeComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/apikeys", `{"name":"revoke-me","user_id":"u1","role":"reader"}`)
	defer resp.Body.Close()
	var keyResp map[string]any
	json.NewDecoder(resp.Body).Decode(&keyResp)
	keyID := keyResp["id"].(string)

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/apikeys/"+keyID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/apikeys/"+keyID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on double revoke, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/apikeys/nonexistent", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestOAuthLoginComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/auth/github/login")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestOAuthCallbackMissingStateComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/auth/github/callback?code=abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestOAuthCallbackNoOAuthComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/auth/github/callback?code=abc&state=teststate", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "teststate"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Dataset Tests
// =============================================================================

func TestDatasetCRUDComprehensive2(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"test-ds","cases":[{"id":"tc-1","input":{"q":"hi"},"expected_contains":["hello"]}]}`
	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created models.TestDataset
	json.NewDecoder(resp.Body).Decode(&created)

	resp = doReq(t, "GET", ts.URL+"/api/v1/datasets/"+created.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err := http.Get(ts.URL + "/api/v1/datasets")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var datasets []models.TestDataset
	json.NewDecoder(resp.Body).Decode(&datasets)
	if len(datasets) != 1 {
		t.Fatalf("expected 1, got %d", len(datasets))
	}

	updateBody := `{"name":"updated-ds","cases":[{"id":"tc-2","input":{"q":"bye"}}]}`
	resp = doReq(t, "PUT", ts.URL+"/api/v1/datasets/"+created.ID, updateBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/datasets/" + created.ID + "/export")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	csvBody := `{"csv":"question,expected:contains\nhi,hello\nbye,goodbye"}`
	resp = doReq(t, "POST", ts.URL+"/api/v1/datasets/"+created.ID+"/import-csv", csvBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doReq(t, "DELETE", ts.URL+"/api/v1/datasets/"+created.ID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDatasetNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/datasets/nonexistent", ""},
		{"DELETE", "/api/v1/datasets/nonexistent", ""},
		{"GET", "/api/v1/datasets/nonexistent/export", ""},
		{"POST", "/api/v1/datasets/nonexistent/import-csv", `{"csv":"a,b"}`},
	}
	for _, tt := range tests {
		resp := doReq(t, tt.method, ts.URL+tt.path, tt.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestDatasetCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets", `{"cases":[]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDatasetUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/datasets/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDatasetImportValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets/import", `{"cases":[]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/datasets/import", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDatasetImportCSVEmptyComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateDataset(ctx, &models.TestDataset{ID: "ds-csv", Name: "t", Cases: []models.TestCase{}, CreatedBy: "u", CreatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets/ds-csv/import-csv", `{"csv":""}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/datasets/ds-csv/import-csv", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDatasetImportCSVNoDataRowsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateDataset(ctx, &models.TestDataset{ID: "ds-csv2", Name: "t", Cases: []models.TestCase{}, CreatedBy: "u", CreatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets/ds-csv2/import-csv", `{"csv":"question,answer"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDatasetUpdateInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateDataset(ctx, &models.TestDataset{ID: "ds-bad", Name: "t", Cases: []models.TestCase{}, CreatedBy: "u", CreatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/datasets/ds-bad", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDatasetExportNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/datasets/nonexistent/export")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDatasetImportCSVNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/datasets/nonexistent/import-csv", `{"csv":"a,b"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Reviews Tests
// =============================================================================

func TestReviewRejectComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "rev-p", Name: "test", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"rev-p","resource_type":"prompt","author":"reviewer"}`)
	var review models.Review
	json.NewDecoder(resp.Body).Decode(&review)
	resp.Body.Close()

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+review.ID+"/reject", bytes.NewReader([]byte(`{"reason":"Needs testing"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestReviewApproveNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/reviews/nonexistent/approve", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestReviewRejectNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/reviews/nonexistent/reject", `{"reason":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestReviewAddCommentNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews/nonexistent/comment", `{"user_id":"u","content":"hi"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestReviewCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"r"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/reviews", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReviewRejectInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "rev-rj", Name: "t", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"rev-rj","resource_type":"prompt","author":"r"}`)
	var review models.Review
	json.NewDecoder(resp.Body).Decode(&review)
	resp.Body.Close()

	resp = doReq(t, "PUT", ts.URL+"/api/v1/reviews/"+review.ID+"/reject", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestReviewAddCommentEmptyContentComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "rev-cm", Name: "t", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"rev-cm","resource_type":"prompt","author":"r"}`)
	var review models.Review
	json.NewDecoder(resp.Body).Decode(&review)
	resp.Body.Close()

	resp = doReq(t, "POST", ts.URL+"/api/v1/reviews/"+review.ID+"/comment", `{"user_id":"u","content":""}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Audit Tests
// =============================================================================

func TestAuditExportCSVComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	db.AppendAudit(ctx, &models.AuditEntry{
		ID: "ae-csv", UserID: "u", Action: "create", Resource: "prompt:p-1",
		Details: map[string]any{"name": "test"}, Timestamp: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/v1/audit/export?format=csv")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected csv, got %q", ct)
	}
}

func TestAuditVerifyChainComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit/verify")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuditListWithFiltersComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	db.AppendAudit(ctx, &models.AuditEntry{
		ID: "ae-f", UserID: "u1", Action: "create", Resource: "prompt:p-1",
		Timestamp: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/v1/audit?user_id=u1&action=create&limit=5")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuditListInvalidSinceComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit?since=not-a-date")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditListInvalidUntilComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit?until=not-a-date")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditExportInvalidSinceComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit/export?since=bad")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditExportInvalidUntilComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit/export?until=bad")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Eval Tests
// =============================================================================

func TestEvalRunComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateDataset(ctx, &models.TestDataset{
		ID: "eval-ds", Name: "basic",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"q": "hello"}, ExpectedContains: []string{"hi"}},
			{ID: "tc-2", Input: map[string]any{"q": "bye"}, ExpectedContains: []string{"goodbye"}},
		},
		CreatedBy: "u", CreatedAt: now,
	})

	resp := doReq(t, "POST", ts.URL+"/api/v1/eval/run", `{"prompt_hash":"hash-abc","dataset_id":"eval-ds","model":"gpt-4"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err := http.Get(ts.URL + "/api/v1/eval/results?prompt_hash=hash-abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/eval/results?dataset_id=eval-ds")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/eval/runs?prompt_hash=hash-abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/eval/report?prompt_hash=hash-abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/eval/compare?a=hash-abc&b=hash-abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestEvalRunValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/eval/run", `{"dataset_id":"d"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/eval/run", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/eval/run", `{"prompt_hash":"h","dataset_id":"nonexistent"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestEvalListResultsValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/results")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEvalReportValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/report")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEvalCompareValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/compare?a=hash1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEvalRunsWithFiltersComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/runs?prompt_hash=h&dataset_id=d&model=gpt-4&status=completed")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =============================================================================
// User Tests
// =============================================================================

func TestUserCreateValidationComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/api/v1/users", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doReq(t, "POST", ts.URL+"/api/v1/users", `{"email":"a@b.com"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}
}

func TestUserUpdateNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "PUT", ts.URL+"/api/v1/users/nonexistent", `{"name":"x"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserUpdateInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateUser(ctx, &models.User{ID: "usr-1", Email: "t@t.com", Name: "t", Role: "reader", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/users/usr-1", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUserDeleteNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp := doReq(t, "DELETE", ts.URL+"/api/v1/users/nonexistent", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// =============================================================================
// Additional edge case tests
// =============================================================================

func TestLogsStreamNoHubComprehensive(t *testing.T) {
	srv := setupTestServerMinimal(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/logs/stream")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPromptUpdateStatusWithReviewComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreatePrompt(ctx, &models.Prompt{ID: "p-rev", Name: "reviewed", Content: "c", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	// Create and approve a review
	resp := doReq(t, "POST", ts.URL+"/api/v1/reviews", `{"resource_id":"p-rev","resource_type":"prompt","author":"reviewer","quorum_required":1}`)
	var review models.Review
	json.NewDecoder(resp.Body).Decode(&review)
	resp.Body.Close()

	resp = doReq(t, "PUT", ts.URL+"/api/v1/reviews/"+review.ID+"/approve", "")
	resp.Body.Close()

	// Now try to approve the prompt directly - should fail since status is draft
	resp = doReq(t, "PUT", ts.URL+"/api/v1/prompts/p-rev", `{"status":"approved"}`)
	defer resp.Body.Close()
	// This should be bad request since draft -> approved needs review (but review already auto-approved it)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 200 or 400, got %d", resp.StatusCode)
	}
}

func TestProviderTestDefaultModelComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Without openai provider configured, expect 400
	resp := doReq(t, "POST", ts.URL+"/api/v1/providers/openai/test", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentUpdateWithStepsComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateAgent(ctx, &models.Agent{ID: "ausv-1", Name: "step-val", Status: models.StatusDraft, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/agents/ausv-1", `{"steps":[{"id":"s1","output_key":"out1"}]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestContextUpdateInvalidJSONComprehensive(t *testing.T) {
	srv, db := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()
	db.CreateContext(ctx, &models.Context{ID: "ctx-bad", Name: "t", CreatedAt: now, UpdatedAt: now})

	resp := doReq(t, "PUT", ts.URL+"/api/v1/contexts/ctx-bad", `{bad`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAgentExportNotFoundComprehensive(t *testing.T) {
	srv, _ := setupTestServerWithDeps(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/agents/nonexistent/export")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
