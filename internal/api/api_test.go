package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/store"
)

func setupTestServer(t *testing.T) (*Server, *store.SQLite) {
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
	return srv, db
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
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
		t.Fatalf("expected status 'healthy', got %v", body["status"])
	}
}

func TestReadyEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
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

func TestPromptCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create
	body := `{"name":"test-prompt","content":"You are helpful.","tags":["test"]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST /api/v1/prompts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created models.Prompt
	json.NewDecoder(resp.Body).Decode(&created)
	if created.Name != "test-prompt" {
		t.Fatalf("expected name 'test-prompt', got %q", created.Name)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// Get
	resp, err = http.Get(ts.URL + "/api/v1/prompts/" + created.ID)
	if err != nil {
		t.Fatalf("GET /api/v1/prompts/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatalf("GET /api/v1/prompts: %v", err)
	}
	defer resp.Body.Close()
	var prompts []models.Prompt
	json.NewDecoder(resp.Body).Decode(&prompts)
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}

	// Update
	updatedBody := `{"name":"updated-prompt","content":"Updated content."}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+created.ID, bytes.NewReader([]byte(updatedBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/prompts/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Delete
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/prompts/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/prompts/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestPromptCreateValidation(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Missing name
	body := `{"content":"hello"}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}

	// Missing content
	body = `{"name":"test"}`
	resp, err = http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing content, got %d", resp.StatusCode)
	}
}

func TestPromptNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/prompts/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAgentCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create
	body := `{"name":"research-agent","description":"Multi-step agent"}`
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

	// Get
	resp, err = http.Get(ts.URL + "/api/v1/agents/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/agents")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var agents []models.Agent
	json.NewDecoder(resp.Body).Decode(&agents)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	// Delete
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/agents/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDatasetCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create
	body := `{"name":"basic-qa","cases":[{"id":"tc-1","input":{"q":"test"},"expected_contains":["answer"]}]}`
	resp, err := http.Post(ts.URL+"/api/v1/datasets", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created models.TestDataset
	json.NewDecoder(resp.Body).Decode(&created)

	// Get
	resp, err = http.Get(ts.URL + "/api/v1/datasets/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/datasets")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var datasets []models.TestDataset
	json.NewDecoder(resp.Body).Decode(&datasets)
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}

	// Update
	updateBody := `{"name":"updated-qa","cases":[{"id":"tc-2","input":{"q":"updated"}}]}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/datasets/"+created.ID, bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d", resp.StatusCode)
	}
	var updated models.TestDataset
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Name != "updated-qa" {
		t.Fatalf("expected name 'updated-qa', got %q", updated.Name)
	}
	if len(updated.Cases) != 1 || updated.Cases[0].ID != "tc-2" {
		t.Fatalf("expected cases with tc-2, got %v", updated.Cases)
	}

	// Export
	resp, err = http.Get(ts.URL + "/api/v1/datasets/" + created.ID + "/export")
	if err != nil {
		t.Fatalf("EXPORT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for export, got %d", resp.StatusCode)
	}

	// Delete
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/datasets/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDatasetImport(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"imported","cases":[{"id":"tc-1","input":{"q":"hi"}}]}`
	resp, err := http.Post(ts.URL+"/api/v1/datasets/import", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST import: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var d models.TestDataset
	json.NewDecoder(resp.Body).Decode(&d)
	if d.Name != "imported" {
		t.Fatalf("expected name 'imported', got %q", d.Name)
	}
	if len(d.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(d.Cases))
	}
}

func TestUserCRUD(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create
	body := `{"email":"alice@example.com","name":"Alice","role":"writer"}`
	resp, err := http.Post(ts.URL+"/api/v1/users", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created models.User
	json.NewDecoder(resp.Body).Decode(&created)
	if created.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %q", created.Email)
	}

	// Get
	resp, err = http.Get(ts.URL + "/api/v1/users/" + created.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var users []models.User
	json.NewDecoder(resp.Body).Decode(&users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	// Update
	updateBody := `{"name":"Alice Updated","role":"admin"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/users/"+created.ID, bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Delete
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/users/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestReviewWorkflow(t *testing.T) {
	srv, db := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create a prompt first
	ctx := context.Background()
	now := time.Now()
	p := &models.Prompt{ID: "p-1", Name: "test", Content: "hello", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p) //nolint:errcheck

	// Create review
	body := `{"resource_id":"p-1","resource_type":"prompt","author":"reviewer"}`
	resp, err := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var review models.Review
	json.NewDecoder(resp.Body).Decode(&review)

	// List pending
	resp, err = http.Get(ts.URL + "/api/v1/reviews")
	if err != nil {
		t.Fatalf("LIST: %v", err)
	}
	defer resp.Body.Close()
	var reviews []models.Review
	json.NewDecoder(resp.Body).Decode(&reviews)
	if len(reviews) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(reviews))
	}

	// Add comment
	commentBody := `{"user_id":"u-1","content":"Looks good!"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/reviews/"+review.ID+"/comment", bytes.NewReader([]byte(commentBody)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST comment: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for comment, got %d", resp.StatusCode)
	}

	// Approve
	req, _ = http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+review.ID+"/approve", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT approve: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for approve, got %d", resp.StatusCode)
	}

	// List pending should be empty now
	resp, err = http.Get(ts.URL + "/api/v1/reviews")
	if err != nil {
		t.Fatalf("LIST after approve: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&reviews)
	if len(reviews) != 0 {
		t.Fatalf("expected 0 pending reviews after approve, got %d", len(reviews))
	}
}

func TestAuditEndpoint(t *testing.T) {
	srv, db := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Add audit entry directly
	ctx := context.Background()
	db.AppendAudit(ctx, &models.AuditEntry{ //nolint:errcheck
		ID: "ae-1", UserID: "u-1", Action: "create", Resource: "prompt:p-1",
		Details: map[string]any{"name": "test"}, Timestamp: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/v1/audit")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var entries []models.AuditEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
}

func TestAutoAuditOnMutations(t *testing.T) {
	srv, _ := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create a prompt — should produce an audit entry
	body := `{"name":"audit-test","content":"hello"}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	var created models.Prompt
	json.NewDecoder(resp.Body).Decode(&created)

	// Give the async goroutine time to write
	time.Sleep(100 * time.Millisecond)

	// Verify audit entry was written
	resp, err = http.Get(ts.URL + "/api/v1/audit?resource=prompt:" + created.ID)
	if err != nil {
		t.Fatalf("GET audit: %v", err)
	}
	defer resp.Body.Close()
	var entries []models.AuditEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry for prompt create, got %d", len(entries))
	}
	if entries[0].Action != "create" {
		t.Fatalf("expected action 'create', got %q", entries[0].Action)
	}
	if entries[0].Resource != "prompt:"+created.ID {
		t.Fatalf("expected resource 'prompt:%s', got %q", created.ID, entries[0].Resource)
	}

	// Update the prompt — should produce another audit entry
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+created.ID, bytes.NewReader([]byte(`{"name":"updated"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err = http.Get(ts.URL + "/api/v1/audit?resource=prompt:" + created.ID + "&action=update")
	if err != nil {
		t.Fatalf("GET audit: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry for prompt update, got %d", len(entries))
	}

	// Delete the prompt — should produce a delete audit entry
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/prompts/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err = http.Get(ts.URL + "/api/v1/audit?resource=prompt:" + created.ID + "&action=delete")
	if err != nil {
		t.Fatalf("GET audit: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry for prompt delete, got %d", len(entries))
	}
}

func TestEvalRunAndReport(t *testing.T) {
	srv, db := setupTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx := context.Background()
	now := time.Now()

	// Seed a prompt
	p := &models.Prompt{ID: "eval-p-1", Name: "test", Content: "hello", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p) //nolint:errcheck

	// Seed a dataset
	d := &models.TestDataset{
		ID:   "eval-ds-1",
		Name: "basic",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"q": "hello"}, ExpectedContains: []string{"hi"}},
			{ID: "tc-2", Input: map[string]any{"q": "bye"}, ExpectedContains: []string{"goodbye"}},
		},
		CreatedBy: "u",
		CreatedAt: now,
	}
	db.CreateDataset(ctx, d) //nolint:errcheck

	// Run eval
	body := `{"prompt_hash":"hash-abc","dataset_id":"eval-ds-1","model":"gpt-4"}`
	resp, err := http.Post(ts.URL+"/api/v1/eval/run", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST eval/run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var report models.EvalReport
	json.NewDecoder(resp.Body).Decode(&report)
	if report.Aggregate.TotalCases != 2 {
		t.Fatalf("expected 2 total cases, got %d", report.Aggregate.TotalCases)
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}

	// List results by prompt hash
	resp, err = http.Get(ts.URL + "/api/v1/eval/results?prompt_hash=hash-abc")
	if err != nil {
		t.Fatalf("GET eval/results: %v", err)
	}
	defer resp.Body.Close()
	var results []models.EvalResult
	json.NewDecoder(resp.Body).Decode(&results)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Get report
	resp, err = http.Get(ts.URL + "/api/v1/eval/report?prompt_hash=hash-abc")
	if err != nil {
		t.Fatalf("GET eval/report: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&report)
	if report.Aggregate.TotalCases != 2 {
		t.Fatalf("expected 2 total cases in report, got %d", report.Aggregate.TotalCases)
	}
}
