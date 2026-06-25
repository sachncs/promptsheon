package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func setupTestDB(t *testing.T) *SQLite {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewSQLiteCreatesDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file should exist")
	}
}

// TestNewSQLite_RejectsUnwritablePath pins the C-1 fix: the previous
// implementation had two identical `if err != nil` blocks after
// sql.Open, the second of which was dead code. The fix is to delete the
// dead block. We re-assert that NewSQLite actually reports an error
// when sql.Open fails, so a future refactor that re-introduces a dead
// block (and accidentally swallows the real one) is caught.
func TestNewSQLite_RejectsUnwritablePath(t *testing.T) {
	// A path inside a non-existent directory should propagate an error
	// from sql.Open or the migration step. Either way, NewSQLite must
	// return a non-nil error and a nil *SQLite.
	dir := t.TempDir()
	bad := filepath.Join(dir, "does", "not", "exist", "x.db")
	db, err := NewSQLite(bad)
	if err == nil {
		if db != nil {
			db.Close()
		}
		t.Fatal("NewSQLite: expected error for unwritable path, got nil")
	}
	if db != nil {
		t.Fatal("NewSQLite: expected nil *SQLite on error, got non-nil")
	}
}

func TestPromptCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	now := time.Now()
	p := &models.Prompt{
		ID:          "p-001",
		Name:        "test-prompt",
		Description: "A test prompt",
		Content:     "You are a helpful assistant.",
		Variables:   []models.Variable{{Name: "topic", Type: "string", Required: true}},
		Tags:        []string{"test", "assistant"},
		ModelHint:   "gpt-4",
		Version:     1,
		Status:      models.StatusDraft,
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]string{"env": "test"},
	}

	// Create
	if err := db.CreatePrompt(ctx, p); err != nil {
		t.Fatalf("CreatePrompt: %v", err)
	}

	// Get
	got, err := db.GetPrompt(ctx, "p-001")
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if got.Name != "test-prompt" {
		t.Fatalf("expected name 'test-prompt', got %q", got.Name)
	}
	if got.Content != "You are a helpful assistant." {
		t.Fatalf("content mismatch")
	}
	if len(got.Variables) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(got.Variables))
	}
	if len(got.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got.Tags))
	}

	// Update
	p.Name = "updated-prompt"
	p.Version = 2
	p.UpdatedAt = time.Now()
	if err := db.UpdatePrompt(ctx, p); err != nil {
		t.Fatalf("UpdatePrompt: %v", err)
	}
	got, _ = db.GetPrompt(ctx, "p-001")
	if got.Name != "updated-prompt" {
		t.Fatalf("expected updated name, got %q", got.Name)
	}
	if got.Version != 2 {
		t.Fatalf("expected version 2, got %d", got.Version)
	}

	// List
	prompts, err := db.ListPrompts(ctx, models.PromptFilter{})
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}

	// List with filter
	prompts, err = db.ListPrompts(ctx, models.PromptFilter{Status: []models.PromptStatus{models.StatusApproved}})
	if err != nil {
		t.Fatalf("ListPrompts filtered: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected 0 approved prompts, got %d", len(prompts))
	}

	// Delete
	if err := db.DeletePrompt(ctx, "p-001"); err != nil {
		t.Fatalf("DeletePrompt: %v", err)
	}
	_, err = db.GetPrompt(ctx, "p-001")
	if err == nil {
		t.Fatal("expected error for deleted prompt")
	}
}

func TestPromptNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.GetPrompt(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent prompt")
	}
}

func TestPromptDeleteNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.DeletePrompt(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent prompt")
	}
}

func TestPromptListWithSearch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.CreatePrompt(ctx, &models.Prompt{ID: "p-1", Name: "greeting", Content: "Hello!", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})
	db.CreatePrompt(ctx, &models.Prompt{ID: "p-2", Name: "farewell", Content: "Goodbye!", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	prompts, err := db.ListPrompts(ctx, models.PromptFilter{Search: "hello"})
	if err != nil {
		t.Fatalf("ListPrompts search: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt matching 'hello', got %d", len(prompts))
	}
	if prompts[0].Name != "greeting" {
		t.Fatalf("expected 'greeting', got %q", prompts[0].Name)
	}
}

func TestPromptListWithTags(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.CreatePrompt(ctx, &models.Prompt{ID: "p-1", Name: "a", Tags: []string{"math", "science"}, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})
	db.CreatePrompt(ctx, &models.Prompt{ID: "p-2", Name: "b", Tags: []string{"history"}, CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	prompts, err := db.ListPrompts(ctx, models.PromptFilter{Tags: []string{"math"}})
	if err != nil {
		t.Fatalf("ListPrompts tags: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt with 'math' tag, got %d", len(prompts))
	}
}

func TestAgentCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	a := &models.Agent{
		ID:          "a-001",
		Name:        "research-agent",
		Description: "Multi-step research",
		Steps: []models.AgentStep{
			{ID: "s1", PromptID: "p-1", DependsOn: nil, OutputKey: "result"},
		},
		Tools:     []models.ToolRef{{Name: "web", Type: models.ToolHTTP}},
		Status:    models.StatusDraft,
		CreatedBy: "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	got, err := db.GetAgent(ctx, "a-001")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Name != "research-agent" {
		t.Fatalf("expected 'research-agent', got %q", got.Name)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}

	agents, err := db.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a.Name = "updated-agent"
	a.UpdatedAt = time.Now()
	if err := db.UpdateAgent(ctx, a); err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}
	got, _ = db.GetAgent(ctx, "a-001")
	if got.Name != "updated-agent" {
		t.Fatalf("expected 'updated-agent', got %q", got.Name)
	}

	if err := db.DeleteAgent(ctx, "a-001"); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}
	_, err = db.GetAgent(ctx, "a-001")
	if err == nil {
		t.Fatal("expected error for deleted agent")
	}
}

func TestDatasetCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	d := &models.TestDataset{
		ID:   "ds-001",
		Name: "basic-qa",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"q": "What is Go?"}, ExpectedContains: []string{"language"}},
		},
		CreatedBy: "user-1",
		CreatedAt: now,
	}

	if err := db.CreateDataset(ctx, d); err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}

	got, err := db.GetDataset(ctx, "ds-001")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if got.Name != "basic-qa" {
		t.Fatalf("expected 'basic-qa', got %q", got.Name)
	}
	if len(got.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(got.Cases))
	}

	datasets, err := db.ListDatasets(ctx)
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}

	if err := db.DeleteDataset(ctx, "ds-001"); err != nil {
		t.Fatalf("DeleteDataset: %v", err)
	}
}

func TestEvalResults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	results := []*models.EvalResult{
		{
			TestCaseID: "tc-1", PromptHash: "hash1", DatasetID: "ds-1", Model: "gpt-4",
			Output: "Go is a language", Score: 0.95, LatencyMs: 200,
			TokenUsage: models.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			Passed:     true, CreatedAt: now,
		},
		{
			TestCaseID: "tc-2", PromptHash: "hash1", DatasetID: "ds-1", Model: "gpt-4",
			Output: "Go is compiled", Score: 0.88, LatencyMs: 180,
			TokenUsage: models.Usage{PromptTokens: 10, CompletionTokens: 15, TotalTokens: 25},
			Passed:     true, CreatedAt: now,
		},
	}

	if err := db.SaveEvalResults(ctx, results); err != nil {
		t.Fatalf("SaveEvalResults: %v", err)
	}

	got, err := db.GetEvalResults(ctx, "hash1")
	if err != nil {
		t.Fatalf("GetEvalResults: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].TokenUsage.TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens, got %d", got[0].TokenUsage.TotalTokens)
	}

	byDataset, err := db.GetEvalResultsByDataset(ctx, "ds-1")
	if err != nil {
		t.Fatalf("GetEvalResultsByDataset: %v", err)
	}
	if len(byDataset) != 2 {
		t.Fatalf("expected 2 results by dataset, got %d", len(byDataset))
	}
}

func TestAuditEntries(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{
		ID:        "ae-001",
		UserID:    "user-1",
		Action:    "create",
		Resource:  "prompt:p-001",
		Details:   map[string]any{"name": "test"},
		Timestamp: now,
	}

	if err := db.AppendAudit(ctx, entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	entries, err := db.ListAudit(ctx, models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != "create" {
		t.Fatalf("expected action 'create', got %q", entries[0].Action)
	}

	// Filter by user
	entries, err = db.ListAudit(ctx, models.AuditFilter{UserID: "user-2"})
	if err != nil {
		t.Fatalf("ListAudit filtered: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for user-2, got %d", len(entries))
	}

	// Filter by action
	entries, err = db.ListAudit(ctx, models.AuditFilter{Action: "delete"})
	if err != nil {
		t.Fatalf("ListAudit action filter: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 delete entries, got %d", len(entries))
	}
}

func TestReviews(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	r := &models.Review{
		ID:           "rv-001",
		ResourceID:   "p-001",
		ResourceType: "prompt",
		Author:       "user-1",
		Status:       models.ReviewPending,
		Comments:     []models.Comment{{ID: "c-1", UserID: "user-2", Content: "LGTM", CreatedAt: now}},
		CreatedAt:    now,
	}

	if err := db.CreateReview(ctx, r); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	got, err := db.GetReview(ctx, "rv-001")
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.Status != models.ReviewPending {
		t.Fatalf("expected pending status, got %q", got.Status)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}

	// List pending
	reviews, err := db.ListPendingReviews(ctx)
	if err != nil {
		t.Fatalf("ListPendingReviews: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(reviews))
	}

	// Approve
	resolvedAt := time.Now()
	r.Status = models.ReviewApproved
	r.ResolvedAt = &resolvedAt
	if err := db.UpdateReview(ctx, r); err != nil {
		t.Fatalf("UpdateReview: %v", err)
	}

	reviews, err = db.ListPendingReviews(ctx)
	if err != nil {
		t.Fatalf("ListPendingReviews after approve: %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("expected 0 pending reviews after approve, got %d", len(reviews))
	}

	// List by resource
	reviews, err = db.ListReviewsByResource(ctx, "p-001", "prompt")
	if err != nil {
		t.Fatalf("ListReviewsByResource: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 review for resource, got %d", len(reviews))
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db1, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("first NewSQLite: %v", err)
	}
	db1.Close()

	db2, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("second NewSQLite (idempotent migration): %v", err)
	}
	db2.Close()
}

func TestPromptListLimitOffset(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		db.CreatePrompt(ctx, &models.Prompt{
			ID:        fmt.Sprintf("p-%d", i),
			Name:      fmt.Sprintf("prompt-%d", i),
			CreatedBy: "u", CreatedAt: now, UpdatedAt: now,
		})
	}

	prompts, err := db.ListPrompts(ctx, models.PromptFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListPrompts limit: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts with limit, got %d", len(prompts))
	}

	prompts, err = db.ListPrompts(ctx, models.PromptFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListPrompts offset: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts with offset, got %d", len(prompts))
	}
}

func TestAPIKeyCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	key := &models.APIKey{
		ID:        "ak-001",
		UserID:    "user-1",
		Name:      "test-key",
		KeyHash:   "hash123",
		KeyPrefix: "ps_abc",
		Role:      "admin",
		CreatedAt: now,
	}

	if err := db.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	// Get by hash.
	got, err := db.GetAPIKeyByHash(ctx, "hash123")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash: %v", err)
	}
	if got == nil {
		t.Fatal("expected to find key by hash")
	}
	if got.UserID != "user-1" {
		t.Fatalf("expected user_id 'user-1', got %q", got.UserID)
	}

	// Get by ID.
	got, err = db.GetAPIKeyByID(ctx, "ak-001")
	if err != nil {
		t.Fatalf("GetAPIKeyByID: %v", err)
	}
	if got.Name != "test-key" {
		t.Fatalf("expected name 'test-key', got %q", got.Name)
	}

	// List by user.
	keys, err := db.ListAPIKeysByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListAPIKeysByUser: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Update last used.
	if err := db.UpdateAPIKeyLastUsed(ctx, "ak-001"); err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed: %v", err)
	}
	got, _ = db.GetAPIKeyByID(ctx, "ak-001")
	if got.LastUsed == nil {
		t.Fatal("expected last_used to be set")
	}

	// Delete (revoke).
	if err := db.DeleteAPIKey(ctx, "ak-001"); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	got, _ = db.GetAPIKeyByHash(ctx, "hash123")
	if got != nil {
		t.Fatal("expected revoked key to not be found by hash")
	}

	// Get by ID still returns revoked keys.
	got, err = db.GetAPIKeyByID(ctx, "ak-001")
	if err != nil {
		t.Fatalf("GetAPIKeyByID after revoke: %v", err)
	}
	if !got.Revoked {
		t.Fatal("expected key to be revoked")
	}
}
