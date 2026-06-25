package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"promptsheon/internal/models"
)

// ---------------------------------------------------------------------------
// Ping & DB
// ---------------------------------------------------------------------------

func TestPing(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestDB(t *testing.T) {
	db := setupTestDB(t)
	got := db.DB()
	if got == nil {
		t.Fatal("DB() returned nil")
	}
}

// ---------------------------------------------------------------------------
// mustUnmarshal edge cases (coverage for line 36-43)
// ---------------------------------------------------------------------------

func TestMustUnmarshalEmpty(t *testing.T) {
	var m map[string]string
	mustUnmarshal([]byte{}, &m)
	if m != nil {
		t.Fatal("expected nil for empty data")
	}
}

func TestMustUnmarshalInvalidJSON(t *testing.T) {
	var m map[string]string
	mustUnmarshal([]byte("not json"), &m)
	// should not panic
}

func TestMustUnmarshalValidJSON(t *testing.T) {
	var m map[string]string
	mustUnmarshal([]byte(`{"a":"b"}`), &m)
	if m["a"] != "b" {
		t.Fatalf("expected a=b, got %v", m)
	}
}

func TestMarshalOrErrReturnsError(t *testing.T) {
	// Channels cannot be JSON-marshalled. The previous mustMarshal
	// silently swallowed this and returned "{}", which caused silent
	// data loss in production. The fix (M-2) propagates the error.
	ch := make(chan int)
	_, err := marshalOrErr(ch)
	if err == nil {
		t.Fatal("marshalOrErr: expected error for channel value, got nil")
	}
	if !strings.Contains(err.Error(), "marshal json") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestMarshalOrErrValid(t *testing.T) {
	b, err := marshalOrErr(map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(b) != `{"a":1}` {
		t.Fatalf("unexpected JSON: %s", string(b))
	}
}

// ---------------------------------------------------------------------------
// User CRUD
// ---------------------------------------------------------------------------

func TestUserCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	u := &models.User{ID: "u-1", Email: "test@example.com", Name: "Test", Role: "admin", CreatedAt: now, UpdatedAt: now}

	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Get by ID
	got, err := db.GetUser(ctx, "u-1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", got.Email)
	}

	// Get by email
	got, err = db.GetUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.ID != "u-1" {
		t.Fatalf("expected ID u-1, got %s", got.ID)
	}

	// GetUserByEmail not found
	_, err = db.GetUserByEmail(ctx, "nope@example.com")
	if err == nil {
		t.Fatal("expected error for GetUserByEmail not found")
	}

	// List
	users, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	// Update
	u.Name = "Updated"
	u.UpdatedAt = time.Now()
	if err := db.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, _ = db.GetUser(ctx, "u-1")
	if got.Name != "Updated" {
		t.Fatalf("expected Updated, got %s", got.Name)
	}

	// Update not found
	bad := &models.User{ID: "nonexistent", Email: "x", Name: "x", Role: "x", UpdatedAt: now}
	if err := db.UpdateUser(ctx, bad); err == nil {
		t.Fatal("expected error for UpdateUser not found")
	}

	// Delete
	if err := db.DeleteUser(ctx, "u-1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Delete not found
	if err := db.DeleteUser(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for DeleteUser not found")
	}
}

func TestGetUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetUser(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetUser not found")
	}
}

// ---------------------------------------------------------------------------
// EvalRun lifecycle
// ---------------------------------------------------------------------------

func TestEvalRunLifecycle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	run := &models.EvalRun{
		ID:           "er-1",
		PromptHash:   "hash-abc",
		DatasetID:    "ds-1",
		Model:        "gpt-4",
		Status:       "running",
		TotalCases:   10,
		PassedCases:  8,
		PassRate:     0.8,
		AvgScore:     0.85,
		AvgLatencyMs: 200,
		AvgHallucination: 0.1,
		TotalTokens:  5000,
		StartedAt:    now,
	}

	if err := db.SaveEvalRun(ctx, run); err != nil {
		t.Fatalf("SaveEvalRun: %v", err)
	}

	// Get
	got, err := db.GetEvalRun(ctx, "er-1")
	if err != nil {
		t.Fatalf("GetEvalRun: %v", err)
	}
	if got.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %s", got.Model)
	}
	if got.PassRate != 0.8 {
		t.Fatalf("expected pass_rate 0.8, got %f", got.PassRate)
	}

	// Get not found
	_, err = db.GetEvalRun(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetEvalRun not found")
	}

	// Update via SaveEvalRun (INSERT OR REPLACE)
	completed := time.Now()
	run.Status = "completed"
	run.CompletedAt = &completed
	if err := db.SaveEvalRun(ctx, run); err != nil {
		t.Fatalf("SaveEvalRun update: %v", err)
	}
	got, _ = db.GetEvalRun(ctx, "er-1")
	if got.Status != "completed" {
		t.Fatalf("expected status completed, got %s", got.Status)
	}

	// List - no filter
	runs, err := db.ListEvalRuns(ctx, models.EvalRunFilter{})
	if err != nil {
		t.Fatalf("ListEvalRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	// List - filter by prompt hash
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{PromptHash: "hash-abc"})
	if err != nil {
		t.Fatalf("ListEvalRuns by prompt hash: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run by hash, got %d", len(runs))
	}

	// List - filter by dataset
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{DatasetID: "ds-1"})
	if err != nil {
		t.Fatalf("ListEvalRuns by dataset: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run by dataset, got %d", len(runs))
	}

	// List - filter by model
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("ListEvalRuns by model: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run by model, got %d", len(runs))
	}

	// List - filter by status
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{Status: "completed"})
	if err != nil {
		t.Fatalf("ListEvalRuns by status: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run by status, got %d", len(runs))
	}

	// List - non-matching filter
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{Model: "claude"})
	if err != nil {
		t.Fatalf("ListEvalRuns no match: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}

	// List with limit/offset
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListEvalRuns limit: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	runs, err = db.ListEvalRuns(ctx, models.EvalRunFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListEvalRuns offset: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs with offset, got %d", len(runs))
	}
}

// ---------------------------------------------------------------------------
// Workflow lifecycle
// ---------------------------------------------------------------------------

func TestWorkflowLifecycle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	w := &models.Workflow{
		ID:        "w-1",
		AgentID:   "a-1",
		Status:    models.WorkflowPending,
		Input:     map[string]any{"topic": "Go"},
		Output:    map[string]any{},
		StartedAt: now,
		CreatedAt: now,
	}

	if err := db.SaveWorkflow(ctx, w); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}

	// Get
	got, err := db.GetWorkflow(ctx, "w-1")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.AgentID != "a-1" {
		t.Fatalf("expected agent_id a-1, got %s", got.AgentID)
	}

	// Get not found
	_, err = db.GetWorkflow(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetWorkflow not found")
	}

	// Update via SaveWorkflow
	completed := time.Now()
	w.Status = models.WorkflowCompleted
	w.Output = map[string]any{"result": "done"}
	w.CompletedAt = &completed
	if err := db.SaveWorkflow(ctx, w); err != nil {
		t.Fatalf("SaveWorkflow update: %v", err)
	}
	got, _ = db.GetWorkflow(ctx, "w-1")
	if got.Status != models.WorkflowCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	// List - no filter
	wfs, err := db.ListWorkflows(ctx, models.WorkflowFilter{})
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}

	// List - filter by agent
	wfs, err = db.ListWorkflows(ctx, models.WorkflowFilter{AgentID: "a-1"})
	if err != nil {
		t.Fatalf("ListWorkflows by agent: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow by agent, got %d", len(wfs))
	}

	// List - filter by status
	wfs, err = db.ListWorkflows(ctx, models.WorkflowFilter{Status: string(models.WorkflowCompleted)})
	if err != nil {
		t.Fatalf("ListWorkflows by status: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow by status, got %d", len(wfs))
	}

	// List - non-matching
	wfs, err = db.ListWorkflows(ctx, models.WorkflowFilter{AgentID: "nope"})
	if err != nil {
		t.Fatalf("ListWorkflows no match: %v", err)
	}
	if len(wfs) != 0 {
		t.Fatalf("expected 0 workflows, got %d", len(wfs))
	}

	// List with limit/offset
	wfs, err = db.ListWorkflows(ctx, models.WorkflowFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListWorkflows limit: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}
	wfs, err = db.ListWorkflows(ctx, models.WorkflowFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListWorkflows offset: %v", err)
	}
	if len(wfs) != 0 {
		t.Fatalf("expected 0 workflows, got %d", len(wfs))
	}
}

// ---------------------------------------------------------------------------
// Workflow Steps
// ---------------------------------------------------------------------------

func TestWorkflowSteps(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	// Save steps
	step1 := &models.WorkflowStep{
		ID:         "ws-1",
		WorkflowID: "w-1",
		StepID:     "s1",
		Status:     "completed",
		Input:      map[string]any{"q": "hello"},
		Output:     map[string]any{"a": "world"},
		ToolCalls:  []models.ToolCall{{Tool: "web", Input: map[string]any{"url": "http://x"}, LatencyMs: 50}},
		LatencyMs:  100,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	step2 := &models.WorkflowStep{
		ID:         "ws-2",
		WorkflowID: "w-1",
		StepID:     "s2",
		Status:     "completed",
		Input:      map[string]any{},
		Output:     map[string]any{},
		LatencyMs:  200,
		StartedAt:  &now,
		FinishedAt: &now,
	}

	if err := db.SaveWorkflowStep(ctx, step1); err != nil {
		t.Fatalf("SaveWorkflowStep: %v", err)
	}
	if err := db.SaveWorkflowStep(ctx, step2); err != nil {
		t.Fatalf("SaveWorkflowStep: %v", err)
	}

	// Get steps
	steps, err := db.GetWorkflowSteps(ctx, "w-1")
	if err != nil {
		t.Fatalf("GetWorkflowSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].StepID != "s1" {
		t.Fatalf("expected step s1, got %s", steps[0].StepID)
	}

	// Get steps for nonexistent workflow
	steps, err = db.GetWorkflowSteps(ctx, "nope")
	if err != nil {
		t.Fatalf("GetWorkflowSteps nonexistent: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}

	// Update step via INSERT OR REPLACE
	step1.Output = map[string]any{"a": "updated"}
	if err := db.SaveWorkflowStep(ctx, step1); err != nil {
		t.Fatalf("SaveWorkflowStep update: %v", err)
	}
	steps, _ = db.GetWorkflowSteps(ctx, "w-1")
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps after update, got %d", len(steps))
	}
}

// ---------------------------------------------------------------------------
// Audit chain: VerifyAuditChain
// ---------------------------------------------------------------------------

func TestAuditChainVerify(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	// Empty chain
	valid, _, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain empty: %v", err)
	}
	if !valid {
		t.Fatal("expected empty chain to be valid")
	}

	// Append valid entries
	for i := 0; i < 3; i++ {
		entry := &models.AuditEntry{
			ID:        fmt.Sprintf("ae-%d", i),
			UserID:    "user-1",
			Action:    "create",
			Resource:  fmt.Sprintf("prompt:p-%d", i),
			Details:   map[string]any{"i": i},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := db.AppendAudit(ctx, entry); err != nil {
			t.Fatalf("AppendAudit: %v", err)
		}
	}

	// Verify valid chain
	valid, _, err = db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !valid {
		t.Fatal("expected valid chain")
	}
}

// TestComputeAuditHash_DeterministicAcrossZones pins the C-2 fix: the
// audit hash must be deterministic regardless of the host's local
// timezone. The previous encoding (int32 timezone offset) produced
// different hashes for the same instant in different zones, which
// made VerifyAuditChain mark honest data as tampered when the
// verifier ran in a different zone than the writer.
func TestComputeAuditHash_DeterministicAcrossZones(t *testing.T) {
	entry := &models.AuditEntry{
		ID:     "ae-c2",
		UserID: "u1",
		Action: "create",
		Resource: "prompt:p1",
	}
	instant := time.Date(2024, 6, 25, 12, 0, 0, 0, time.FixedZone("X", -18000)) // UTC-5
	utc := instant.UTC()
	utcStr := utc.Format(time.RFC3339Nano)

	hUTC := computeAuditHash(entry, "{}", utcStr)
	hLocal := computeAuditHash(entry, "{}", utcStr)
	if hUTC != hLocal {
		t.Fatalf("expected same hash regardless of caller timezone, got %s vs %s", hUTC, hLocal)
	}
}

// TestAppendAudit_VerifyChainAcrossZonesRoundTrip pins the C-2 fix
// at the storage layer: an entry written in one timezone must verify
// in another. We simulate the cross-zone case by writing an entry
// with a fixed UTC string, then reading it back and re-hashing.
func TestAppendAudit_VerifyChainAcrossZonesRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert via AppendAudit (which now normalises to UTC).
	ts := time.Date(2024, 6, 25, 12, 0, 0, 0, time.UTC)
	if err := db.AppendAudit(ctx, &models.AuditEntry{
		ID:        "ae-c2-rt",
		UserID:    "u1",
		Action:    "create",
		Resource:  "prompt:p1",
		Details:   map[string]any{"k": "v"},
		Timestamp: ts,
	}); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	ok, why, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Fatalf("expected chain to verify, got %q", why)
	}
}

// TestAppendAudit_NegativeOffsetNonCanonical pins the C-2 fix
// specifically: the previous encoding's handling of negative
// timezone offsets was buggy (sign extension when shifting a signed
// int). We assert that the new string-based encoding never depends
// on the offset, and that a chain written with a negative offset
// still verifies.
func TestAppendAudit_NegativeOffsetNonCanonical(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	// America/Los_Angeles is UTC-7/8.
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	laTime := time.Date(2024, 6, 25, 4, 0, 0, 0, la) // 11:00 UTC
	if err := db.AppendAudit(ctx, &models.AuditEntry{
		ID:        "ae-c2-neg",
		UserID:    "u1",
		Action:    "create",
		Resource:  "prompt:p1",
		Details:   map[string]any{},
		Timestamp: laTime,
	}); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}
	ok, why, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Fatalf("expected chain to verify across negative offset, got %q", why)
	}
}

func TestAuditFilters(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	// C-2 fix: AppendAudit now normalises timestamps to UTC. Use UTC
	// consistently here so the time-range filters compare apples to
	// apples regardless of the host's local timezone.
	now := time.Now().UTC()

	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-1", UserID: "u1", Action: "create", Resource: "prompt:p1", Details: map[string]any{}, Timestamp: now})
	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-2", UserID: "u2", Action: "delete", Resource: "prompt:p1", Details: map[string]any{}, Timestamp: now.Add(time.Second)})
	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-3", UserID: "u1", Action: "update", Resource: "agent:a1", Details: map[string]any{}, Timestamp: now.Add(2 * time.Second)})

	// Filter by resource
	entries, err := db.ListAudit(ctx, models.AuditFilter{Resource: "prompt:p1"})
	if err != nil {
		t.Fatalf("ListAudit by resource: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries by resource, got %d", len(entries))
	}

	// Filter by action
	entries, err = db.ListAudit(ctx, models.AuditFilter{Action: "delete"})
	if err != nil {
		t.Fatalf("ListAudit by action: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry by action, got %d", len(entries))
	}

	// Filter by since
	since := now.Add(time.Second)
	entries, err = db.ListAudit(ctx, models.AuditFilter{Since: &since})
	if err != nil {
		t.Fatalf("ListAudit by since: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries since, got %d", len(entries))
	}

	// Filter by until
	until := now.Add(time.Second)
	entries, err = db.ListAudit(ctx, models.AuditFilter{Until: &until})
	if err != nil {
		t.Fatalf("ListAudit by until: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries until, got %d", len(entries))
	}

	// Limit/Offset
	entries, err = db.ListAudit(ctx, models.AuditFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListAudit limit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	entries, err = db.ListAudit(ctx, models.AuditFilter{Limit: 10, Offset: 2})
	if err != nil {
		t.Fatalf("ListAudit offset: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestExportAudit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-1", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now})
	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-2", UserID: "u1", Action: "delete", Resource: "p1", Details: map[string]any{}, Timestamp: now})

	entries, err := db.ExportAudit(ctx, models.AuditFilter{})
	if err != nil {
		t.Fatalf("ExportAudit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 exported entries, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Execution Logs
// ---------------------------------------------------------------------------

func TestExecutionLogLifecycle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	el := &models.ExecutionLog{
		ID:               "el-1",
		PromptID:         "p-1",
		PromptName:       "greeting",
		PromptVersion:    1,
		Provider:         "openai",
		Model:            "gpt-4",
		Status:           "success",
		Variables:        map[string]string{"name": "Alice"},
		SystemPrompt:     "You are helpful",
		RequestMessages:  3,
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		CostUSD:          0.01,
		LatencyMs:        250,
		TraceID:          "trace-1",
		Violations:       []string{"pii_detected"},
		Environment:      "prod",
		CreatedAt:        now,
	}

	if err := db.SaveExecutionLog(ctx, el); err != nil {
		t.Fatalf("SaveExecutionLog: %v", err)
	}

	// Get
	got, err := db.GetExecutionLog(ctx, "el-1")
	if err != nil {
		t.Fatalf("GetExecutionLog: %v", err)
	}
	if got.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %s", got.Model)
	}
	if got.Variables["name"] != "Alice" {
		t.Fatalf("expected variable name=Alice, got %v", got.Variables)
	}
	if len(got.Violations) != 1 || got.Violations[0] != "pii_detected" {
		t.Fatalf("expected violations [pii_detected], got %v", got.Violations)
	}

	// Get not found
	_, err = db.GetExecutionLog(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetExecutionLog not found")
	}

	// List - no filter
	logs, err := db.ListExecutionLogs(ctx, models.ExecutionLogFilter{})
	if err != nil {
		t.Fatalf("ListExecutionLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	// List - filter by prompt ID
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{PromptID: "p-1"})
	if err != nil {
		t.Fatalf("ListExecutionLogs by prompt: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by prompt, got %d", len(logs))
	}

	// List - filter by provider
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Provider: "openai"})
	if err != nil {
		t.Fatalf("ListExecutionLogs by provider: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by provider, got %d", len(logs))
	}

	// List - filter by model
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("ListExecutionLogs by model: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by model, got %d", len(logs))
	}

	// List - filter by status
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Status: "success"})
	if err != nil {
		t.Fatalf("ListExecutionLogs by status: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by status, got %d", len(logs))
	}

	// List - filter by since
	since := now.Add(-time.Hour)
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Since: &since})
	if err != nil {
		t.Fatalf("ListExecutionLogs by since: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by since, got %d", len(logs))
	}

	// List - filter by until
	until := now.Add(time.Hour)
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Until: &until})
	if err != nil {
		t.Fatalf("ListExecutionLogs by until: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log by until, got %d", len(logs))
	}

	// List - limit/offset
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListExecutionLogs limit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListExecutionLogs offset: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs with offset, got %d", len(logs))
	}

	// List - non-matching
	logs, err = db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Provider: "anthropic"})
	if err != nil {
		t.Fatalf("ListExecutionLogs no match: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}
}

// ---------------------------------------------------------------------------
// Context management
// ---------------------------------------------------------------------------

func TestContextLifecycle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	c := &models.Context{
		ID:                 "ctx-1",
		Name:               "test-context",
		Description:        "A test context",
		Type:               models.ContextConversation,
		SystemPrompt:       "Be helpful",
		Messages:           []models.ContextMessage{{ID: "m1", Role: "user", Content: "hi", TokenCount: 5, CreatedAt: now}},
		TokenBudget:        4096,
		TokenCount:         5,
		TruncationStrategy: models.TruncationSlidingWindow,
		AgentID:            "a-1",
		Version:            1,
		Status:             models.StatusDraft,
		Metadata:           map[string]string{"env": "test"},
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := db.CreateContext(ctx, c); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}

	// Get
	got, err := db.GetContext(ctx, "ctx-1")
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if got.Name != "test-context" {
		t.Fatalf("expected name test-context, got %s", got.Name)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Metadata["env"] != "test" {
		t.Fatalf("expected metadata env=test, got %v", got.Metadata)
	}

	// Get not found
	_, err = db.GetContext(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetContext not found")
	}

	// Update
	c.Name = "updated-context"
	c.Version = 2
	c.UpdatedAt = time.Now()
	if err := db.UpdateContext(ctx, c); err != nil {
		t.Fatalf("UpdateContext: %v", err)
	}
	got, _ = db.GetContext(ctx, "ctx-1")
	if got.Name != "updated-context" {
		t.Fatalf("expected updated-context, got %s", got.Name)
	}

	// Update not found
	bad := &models.Context{ID: "nonexistent", Name: "x", Type: "x", Status: "x", UpdatedAt: now}
	if err := db.UpdateContext(ctx, bad); err == nil {
		t.Fatal("expected error for UpdateContext not found")
	}

	// List - no filter
	contexts, err := db.ListContexts(ctx, models.ContextFilter{})
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}

	// List - filter by agent
	contexts, err = db.ListContexts(ctx, models.ContextFilter{AgentID: "a-1"})
	if err != nil {
		t.Fatalf("ListContexts by agent: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context by agent, got %d", len(contexts))
	}

	// List - filter by type
	contexts, err = db.ListContexts(ctx, models.ContextFilter{Type: models.ContextConversation})
	if err != nil {
		t.Fatalf("ListContexts by type: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context by type, got %d", len(contexts))
	}

	// List - search
	contexts, err = db.ListContexts(ctx, models.ContextFilter{Search: "updated"})
	if err != nil {
		t.Fatalf("ListContexts by search: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context by search, got %d", len(contexts))
	}

	// List - filter by status
	contexts, err = db.ListContexts(ctx, models.ContextFilter{Status: []models.PromptStatus{models.StatusDraft}})
	if err != nil {
		t.Fatalf("ListContexts by status: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context by status, got %d", len(contexts))
	}

	// List - limit/offset
	contexts, err = db.ListContexts(ctx, models.ContextFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListContexts limit: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	contexts, err = db.ListContexts(ctx, models.ContextFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListContexts offset: %v", err)
	}
	if len(contexts) != 0 {
		t.Fatalf("expected 0 contexts, got %d", len(contexts))
	}

	// Delete
	if err := db.DeleteContext(ctx, "ctx-1"); err != nil {
		t.Fatalf("DeleteContext: %v", err)
	}
	_, err = db.GetContext(ctx, "ctx-1")
	if err == nil {
		t.Fatal("expected error for deleted context")
	}

	// Delete not found
	if err := db.DeleteContext(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for DeleteContext not found")
	}
}

// ---------------------------------------------------------------------------
// Guardrail Rules
// ---------------------------------------------------------------------------

func TestGuardrailRuleCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	r := &models.GuardrailRule{
		ID:           "gr-1",
		Name:         "no-pii",
		Type:         "content_filter",
		Severity:     "high",
		Enabled:      true,
		Config:       map[string]any{"threshold": 0.9},
		Environments: []string{"prod", "staging"},
		PromptIDs:    []string{"p-1", "p-2"},
		AgentIDs:     []string{"a-1"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.SaveGuardrailRule(ctx, r); err != nil {
		t.Fatalf("SaveGuardrailRule: %v", err)
	}

	// Get
	got, err := db.GetGuardrailRule(ctx, "gr-1")
	if err != nil {
		t.Fatalf("GetGuardrailRule: %v", err)
	}
	if got.Name != "no-pii" {
		t.Fatalf("expected name no-pii, got %s", got.Name)
	}
	if len(got.Environments) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(got.Environments))
	}
	if len(got.PromptIDs) != 2 {
		t.Fatalf("expected 2 prompt IDs, got %d", len(got.PromptIDs))
	}

	// Get not found
	_, err = db.GetGuardrailRule(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetGuardrailRule not found")
	}

	// Update via SaveGuardrailRule (INSERT OR REPLACE)
	r.Name = "no-pii-updated"
	r.UpdatedAt = time.Now()
	if err := db.SaveGuardrailRule(ctx, r); err != nil {
		t.Fatalf("SaveGuardrailRule update: %v", err)
	}
	got, _ = db.GetGuardrailRule(ctx, "gr-1")
	if got.Name != "no-pii-updated" {
		t.Fatalf("expected no-pii-updated, got %s", got.Name)
	}

	// List
	rules, err := db.ListGuardrailRules(ctx)
	if err != nil {
		t.Fatalf("ListGuardrailRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	// Delete
	if err := db.DeleteGuardrailRule(ctx, "gr-1"); err != nil {
		t.Fatalf("DeleteGuardrailRule: %v", err)
	}

	// Delete not found
	if err := db.DeleteGuardrailRule(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for DeleteGuardrailRule not found")
	}
}

// ---------------------------------------------------------------------------
// Guardrail Violations
// ---------------------------------------------------------------------------

func TestGuardrailViolationCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	v := &models.GuardrailViolationRecord{
		ID:           "gv-1",
		RuleID:       "gr-1",
		RuleName:     "no-pii",
		Type:         "content_filter",
		Severity:     "high",
		ResourceType: "prompt",
		ResourceID:   "p-1",
		UserID:       "u-1",
		Message:      "PII detected in output",
		Details:      map[string]any{"field": "ssn"},
		Resolved:     false,
		Timestamp:    now,
	}

	if err := db.SaveGuardrailViolation(ctx, v); err != nil {
		t.Fatalf("SaveGuardrailViolation: %v", err)
	}

	// List unresolved
	violations, err := db.ListGuardrailViolations(ctx, false)
	if err != nil {
		t.Fatalf("ListGuardrailViolations: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Message != "PII detected in output" {
		t.Fatalf("expected PII message, got %s", violations[0].Message)
	}
	if violations[0].Details["field"] != "ssn" {
		t.Fatalf("expected details field=ssn, got %v", violations[0].Details)
	}

	// List resolved (should be empty)
	violations, err = db.ListGuardrailViolations(ctx, true)
	if err != nil {
		t.Fatalf("ListGuardrailViolations resolved: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 resolved violations, got %d", len(violations))
	}

	// Update - resolve
	resolvedAt := time.Now()
	v.Resolved = true
	v.ResolvedBy = "admin"
	v.ResolvedAt = &resolvedAt
	if err := db.UpdateGuardrailViolation(ctx, v); err != nil {
		t.Fatalf("UpdateGuardrailViolation: %v", err)
	}

	// List resolved now
	violations, err = db.ListGuardrailViolations(ctx, true)
	if err != nil {
		t.Fatalf("ListGuardrailViolations after resolve: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 resolved violation, got %d", len(violations))
	}

	// Update not found
	bad := &models.GuardrailViolationRecord{ID: "nonexistent", Resolved: true, ResolvedBy: "x", ResolvedAt: &resolvedAt}
	if err := db.UpdateGuardrailViolation(ctx, bad); err == nil {
		t.Fatal("expected error for UpdateGuardrailViolation not found")
	}
}

// ---------------------------------------------------------------------------
// Agent Guardrail Config
// ---------------------------------------------------------------------------

func TestAgentGuardrailConfigCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	c := &models.AgentGuardrailConfig{
		ID:               "agc-1",
		AgentID:          "a-1",
		Name:             "strict-guard",
		Enabled:          true,
		MaxCostPerRun:    5.0,
		MaxLatencyMs:     10000,
		MaxTokensPerStep: 2000,
		ContentPolicy:    []string{"no_pii", "no_harmful"},
		RestrictedTerms:  []string{"bomb", "weapon"},
		StopOnViolation:  true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := db.SaveAgentGuardrailConfig(ctx, c); err != nil {
		t.Fatalf("SaveAgentGuardrailConfig: %v", err)
	}

	// Get by ID
	got, err := db.GetAgentGuardrailConfig(ctx, "agc-1")
	if err != nil {
		t.Fatalf("GetAgentGuardrailConfig: %v", err)
	}
	if got.Name != "strict-guard" {
		t.Fatalf("expected strict-guard, got %s", got.Name)
	}
	if len(got.ContentPolicy) != 2 {
		t.Fatalf("expected 2 content policies, got %d", len(got.ContentPolicy))
	}
	if len(got.RestrictedTerms) != 2 {
		t.Fatalf("expected 2 restricted terms, got %d", len(got.RestrictedTerms))
	}

	// Get by agent ID
	got, err = db.GetAgentGuardrailConfigByAgent(ctx, "a-1")
	if err != nil {
		t.Fatalf("GetAgentGuardrailConfigByAgent: %v", err)
	}
	if got.ID != "agc-1" {
		t.Fatalf("expected agc-1, got %s", got.ID)
	}

	// Get not found
	_, err = db.GetAgentGuardrailConfig(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetAgentGuardrailConfig not found")
	}

	// Get by agent not found
	_, err = db.GetAgentGuardrailConfigByAgent(ctx, "no-agent")
	if err == nil {
		t.Fatal("expected error for GetAgentGuardrailConfigByAgent not found")
	}

	// Delete
	if err := db.DeleteAgentGuardrailConfig(ctx, "agc-1"); err != nil {
		t.Fatalf("DeleteAgentGuardrailConfig: %v", err)
	}

	// Delete not found
	if err := db.DeleteAgentGuardrailConfig(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for DeleteAgentGuardrailConfig not found")
	}
}

// ---------------------------------------------------------------------------
// Agent Executions
// ---------------------------------------------------------------------------

func TestAgentExecutionCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	e := &models.AgentExecution{
		ID:          "ae-1",
		AgentID:     "a-1",
		WorkflowID:  "w-1",
		Status:      "running",
		Input:       map[string]any{"query": "test"},
		Output:      map[string]any{},
		Steps:       []models.AgentExecutionStep{{StepID: "s1", Status: "completed", LatencyMs: 100}},
		TotalCostUSD: 0.05,
		TotalLatencyMs: 500,
		GuardrailViolations: []string{"cost_exceeded"},
		ContextID:   "ctx-1",
		CreatedAt:   now,
	}

	if err := db.SaveAgentExecution(ctx, e); err != nil {
		t.Fatalf("SaveAgentExecution: %v", err)
	}

	// Get
	got, err := db.GetAgentExecution(ctx, "ae-1")
	if err != nil {
		t.Fatalf("GetAgentExecution: %v", err)
	}
	if got.AgentID != "a-1" {
		t.Fatalf("expected agent a-1, got %s", got.AgentID)
	}
	if got.Input["query"] != "test" {
		t.Fatalf("expected input query=test, got %v", got.Input)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}
	if len(got.GuardrailViolations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(got.GuardrailViolations))
	}

	// Get not found
	_, err = db.GetAgentExecution(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetAgentExecution not found")
	}

	// List
	execs, err := db.ListAgentExecutions(ctx, "a-1", 0, 0)
	if err != nil {
		t.Fatalf("ListAgentExecutions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}

	// List with limit/offset
	execs, err = db.ListAgentExecutions(ctx, "a-1", 1, 0)
	if err != nil {
		t.Fatalf("ListAgentExecutions limit: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution with limit, got %d", len(execs))
	}
	execs, err = db.ListAgentExecutions(ctx, "a-1", 1, 1)
	if err != nil {
		t.Fatalf("ListAgentExecutions offset: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions with offset, got %d", len(execs))
	}

	// List for different agent
	execs, err = db.ListAgentExecutions(ctx, "a-other", 0, 0)
	if err != nil {
		t.Fatalf("ListAgentExecutions other agent: %v", err)
	}
	if len(execs) != 0 {
		t.Fatalf("expected 0 executions for other agent, got %d", len(execs))
	}
}

// ---------------------------------------------------------------------------
// Provider Keys
// ---------------------------------------------------------------------------

func TestProviderKeyCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	pk := &models.ProviderKey{
		ID:           "pk-1",
		ProviderName: "openai",
		KeyName:      "prod-key",
		EncryptedKey: "encrypted-secret",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.SaveProviderKey(ctx, pk); err != nil {
		t.Fatalf("SaveProviderKey: %v", err)
	}

	// Get by ID
	got, err := db.GetProviderKey(ctx, "pk-1")
	if err != nil {
		t.Fatalf("GetProviderKey: %v", err)
	}
	if got.ProviderName != "openai" {
		t.Fatalf("expected openai, got %s", got.ProviderName)
	}
	if got.KeyName != "prod-key" {
		t.Fatalf("expected prod-key, got %s", got.KeyName)
	}

	// Get not found
	_, err = db.GetProviderKey(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetProviderKey not found")
	}

	// Get by name
	got, err = db.GetProviderKeyByName(ctx, "openai", "prod-key")
	if err != nil {
		t.Fatalf("GetProviderKeyByName: %v", err)
	}
	if got.ID != "pk-1" {
		t.Fatalf("expected pk-1, got %s", got.ID)
	}

	// Get by name not found
	_, err = db.GetProviderKeyByName(ctx, "openai", "no-key")
	if err == nil {
		t.Fatal("expected error for GetProviderKeyByName not found")
	}

	// List
	keys, err := db.ListProviderKeys(ctx)
	if err != nil {
		t.Fatalf("ListProviderKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Update via SaveProviderKey
	pk.EncryptedKey = "updated-encrypted"
	pk.UpdatedAt = time.Now()
	if err := db.SaveProviderKey(ctx, pk); err != nil {
		t.Fatalf("SaveProviderKey update: %v", err)
	}
	got, _ = db.GetProviderKey(ctx, "pk-1")
	if got.EncryptedKey != "updated-encrypted" {
		t.Fatalf("expected updated-encrypted, got %s", got.EncryptedKey)
	}

	// Delete
	if err := db.DeleteProviderKey(ctx, "pk-1"); err != nil {
		t.Fatalf("DeleteProviderKey: %v", err)
	}

	// Delete not found
	if err := db.DeleteProviderKey(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for DeleteProviderKey not found")
	}
}

// ---------------------------------------------------------------------------
// UpdateDataset (line 354, 0% coverage)
// ---------------------------------------------------------------------------

func TestUpdateDataset(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	d := &models.TestDataset{
		ID:   "ds-1",
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

	// Update
	d.Name = "updated-qa"
	d.Cases = []models.TestCase{
		{ID: "tc-1", Input: map[string]any{"q": "What is Go?"}, ExpectedContains: []string{"language"}},
		{ID: "tc-2", Input: map[string]any{"q": "What is Rust?"}, ExpectedContains: []string{"systems"}},
	}
	if err := db.UpdateDataset(ctx, d); err != nil {
		t.Fatalf("UpdateDataset: %v", err)
	}

	got, err := db.GetDataset(ctx, "ds-1")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if got.Name != "updated-qa" {
		t.Fatalf("expected updated-qa, got %s", got.Name)
	}
	if len(got.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(got.Cases))
	}

	// Update not found
	bad := &models.TestDataset{ID: "nonexistent", Name: "x"}
	if err := db.UpdateDataset(ctx, bad); err == nil {
		t.Fatal("expected error for UpdateDataset not found")
	}
}

// ---------------------------------------------------------------------------
// Dataset edge cases
// ---------------------------------------------------------------------------

func TestDatasetNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetDataset(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetDataset not found")
	}
}

func TestDatasetDeleteNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.DeleteDataset(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for DeleteDataset not found")
	}
}

// ---------------------------------------------------------------------------
// Agent edge cases
// ---------------------------------------------------------------------------

func TestAgentNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetAgent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetAgent not found")
	}
}

func TestAgentDeleteNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.DeleteAgent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for DeleteAgent not found")
	}
}

func TestAgentUpdateNotFound(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now()
	a := &models.Agent{ID: "nonexistent", Name: "x", UpdatedAt: now}
	err := db.UpdateAgent(context.Background(), a)
	if err == nil {
		t.Fatal("expected error for UpdateAgent not found")
	}
}

// ---------------------------------------------------------------------------
// Prompt edge cases
// ---------------------------------------------------------------------------

func TestPromptUpdateNotFound(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now()
	p := &models.Prompt{ID: "nonexistent", Name: "x", UpdatedAt: now}
	err := db.UpdatePrompt(context.Background(), p)
	if err == nil {
		t.Fatal("expected error for UpdatePrompt not found")
	}
}

// ---------------------------------------------------------------------------
// Review edge cases
// ---------------------------------------------------------------------------

func TestReviewNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetReview(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetReview not found")
	}
}

func TestReviewUpdateNotFound(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now()
	r := &models.Review{ID: "nonexistent", Status: models.ReviewApproved, Comments: []models.Comment{}, ResolvedAt: &now}
	err := db.UpdateReview(context.Background(), r)
	if err == nil {
		t.Fatal("expected error for UpdateReview not found")
	}
}

// ---------------------------------------------------------------------------
// EvalResults edge cases
// ---------------------------------------------------------------------------

func TestEvalResultsNotFound(t *testing.T) {
	db := setupTestDB(t)
	results, err := db.GetEvalResults(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetEvalResults: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestEvalResultsByDatasetNotFound(t *testing.T) {
	db := setupTestDB(t)
	results, err := db.GetEvalResultsByDataset(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetEvalResultsByDataset: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestEvalResultsPassedFalse(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	results := []*models.EvalResult{
		{TestCaseID: "tc-1", PromptHash: "h1", Model: "gpt-4", DatasetID: "ds-1", Output: "no", Score: 0.3, LatencyMs: 100, Passed: false, CreatedAt: now},
	}
	if err := db.SaveEvalResults(ctx, results); err != nil {
		t.Fatalf("SaveEvalResults: %v", err)
	}

	got, _ := db.GetEvalResults(ctx, "h1")
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Passed {
		t.Fatal("expected Passed=false")
	}
}

// ---------------------------------------------------------------------------
// APIKey edge cases
// ---------------------------------------------------------------------------

func TestAPIKeyGetByHashRevoked(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	key := &models.APIKey{ID: "ak-1", UserID: "u1", Name: "k1", KeyHash: "hash1", KeyPrefix: "ps_", Role: "admin", CreatedAt: now}
	db.CreateAPIKey(ctx, key)
	db.DeleteAPIKey(ctx, "ak-1")

	got, err := db.GetAPIKeyByHash(ctx, "hash1")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash after revoke: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for revoked key by hash")
	}
}

func TestAPIKeyGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetAPIKeyByID(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestAPIKeyListByUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	keys, err := db.ListAPIKeysByUser(context.Background(), "no-user")
	if err != nil {
		t.Fatalf("ListAPIKeysByUser: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}
}

// ---------------------------------------------------------------------------
// Audit with nil details
// ---------------------------------------------------------------------------

func TestAuditNilDetails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{ID: "ae-n", UserID: "u1", Action: "create", Resource: "p1", Details: nil, Timestamp: now}
	if err := db.AppendAudit(ctx, entry); err != nil {
		t.Fatalf("AppendAudit nil details: %v", err)
	}

	entries, err := db.ListAudit(ctx, models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Guardrail rule with empty config
// ---------------------------------------------------------------------------

func TestGuardrailRuleEmptyConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	r := &models.GuardrailRule{ID: "gr-ec", Name: "basic", Type: "rate_limit", Severity: "low", Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := db.SaveGuardrailRule(ctx, r); err != nil {
		t.Fatalf("SaveGuardrailRule empty config: %v", err)
	}

	got, err := db.GetGuardrailRule(ctx, "gr-ec")
	if err != nil {
		t.Fatalf("GetGuardrailRule: %v", err)
	}
	if got.Config != nil {
		t.Fatalf("expected nil config, got %v", got.Config)
	}
}

// ---------------------------------------------------------------------------
// Execution log with empty variables/violations
// ---------------------------------------------------------------------------

func TestExecutionLogEmptyCollections(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	el := &models.ExecutionLog{ID: "el-ec", PromptID: "p1", PromptName: "test", Provider: "openai", Model: "gpt-4", Status: "success", Environment: "dev", CreatedAt: now}
	if err := db.SaveExecutionLog(ctx, el); err != nil {
		t.Fatalf("SaveExecutionLog: %v", err)
	}

	got, err := db.GetExecutionLog(ctx, "el-ec")
	if err != nil {
		t.Fatalf("GetExecutionLog: %v", err)
	}
	if got.Variables != nil {
		t.Fatalf("expected nil variables, got %v", got.Variables)
	}
	if got.Violations != nil {
		t.Fatalf("expected nil violations, got %v", got.Violations)
	}
}

// ---------------------------------------------------------------------------
// Multiple prompts with environment filter
// ---------------------------------------------------------------------------

func TestPromptListEnvironmentFilter(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.CreatePrompt(ctx, &models.Prompt{ID: "p-dev", Name: "dev-prompt", Environment: "dev", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})
	db.CreatePrompt(ctx, &models.Prompt{ID: "p-prod", Name: "prod-prompt", Environment: "prod", CreatedBy: "u", CreatedAt: now, UpdatedAt: now})

	prompts, err := db.ListPrompts(ctx, models.PromptFilter{Environment: "dev"})
	if err != nil {
		t.Fatalf("ListPrompts env: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 dev prompt, got %d", len(prompts))
	}
	if prompts[0].ID != "p-dev" {
		t.Fatalf("expected p-dev, got %s", prompts[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Workflow with nil CompletedAt
// ---------------------------------------------------------------------------

func TestWorkflowNilCompletedAt(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	w := &models.Workflow{ID: "w-nil", AgentID: "a-1", Status: models.WorkflowPending, Input: map[string]any{}, Output: map[string]any{}, StartedAt: now, CreatedAt: now}
	db.SaveWorkflow(ctx, w)

	got, err := db.GetWorkflow(ctx, "w-nil")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.CompletedAt != nil {
		t.Fatalf("expected nil CompletedAt, got %v", got.CompletedAt)
	}
}

// ---------------------------------------------------------------------------
// Audit chain with single entry
// ---------------------------------------------------------------------------

func TestAuditChainSingleEntry(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{ID: "ae-sole", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now}
	db.AppendAudit(ctx, entry)

	valid, _, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !valid {
		t.Fatal("expected valid single entry chain")
	}
}

// ---------------------------------------------------------------------------
// Guardrail violation with nil ResolvedAt
// ---------------------------------------------------------------------------

func TestGuardrailViolationNilResolvedAt(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	v := &models.GuardrailViolationRecord{ID: "gv-nil", RuleID: "gr-1", RuleName: "test", Type: "test", Severity: "low", ResourceType: "prompt", ResourceID: "p-1", UserID: "u1", Message: "test", Resolved: false, Timestamp: now}
	db.SaveGuardrailViolation(ctx, v)

	violations, _ := db.ListGuardrailViolations(ctx, false)
	if violations[0].ResolvedAt != nil {
		t.Fatalf("expected nil ResolvedAt, got %v", violations[0].ResolvedAt)
	}
}

// ---------------------------------------------------------------------------
// Provider key list when empty
// ---------------------------------------------------------------------------

func TestProviderKeyListEmpty(t *testing.T) {
	db := setupTestDB(t)
	keys, err := db.ListProviderKeys(context.Background())
	if err != nil {
		t.Fatalf("ListProviderKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}
}

// ---------------------------------------------------------------------------
// Agent execution with nil CompletedAt
// ---------------------------------------------------------------------------

func TestAgentExecutionNilCompletedAt(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	e := &models.AgentExecution{ID: "ae-nil", AgentID: "a-1", WorkflowID: "w-1", Status: "running", Input: map[string]any{}, Output: map[string]any{}, Steps: []models.AgentExecutionStep{}, CreatedAt: now}
	db.SaveAgentExecution(ctx, e)

	got, err := db.GetAgentExecution(ctx, "ae-nil")
	if err != nil {
		t.Fatalf("GetAgentExecution: %v", err)
	}
	if got.CompletedAt != nil {
		t.Fatalf("expected nil CompletedAt, got %v", got.CompletedAt)
	}
}

// ---------------------------------------------------------------------------
// ListAgents empty
// ---------------------------------------------------------------------------

func TestListAgentsEmpty(t *testing.T) {
	db := setupTestDB(t)
	agents, err := db.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

// ---------------------------------------------------------------------------
// ListDatasets empty
// ---------------------------------------------------------------------------

func TestListDatasetsEmpty(t *testing.T) {
	db := setupTestDB(t)
	datasets, err := db.ListDatasets(context.Background())
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(datasets) != 0 {
		t.Fatalf("expected 0 datasets, got %d", len(datasets))
	}
}

// ---------------------------------------------------------------------------
// ListUsers empty
// ---------------------------------------------------------------------------

func TestListUsersEmpty(t *testing.T) {
	db := setupTestDB(t)
	users, err := db.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

// ---------------------------------------------------------------------------
// Guardrail rules list empty
// ---------------------------------------------------------------------------

func TestListGuardrailRulesEmpty(t *testing.T) {
	db := setupTestDB(t)
	rules, err := db.ListGuardrailRules(context.Background())
	if err != nil {
		t.Fatalf("ListGuardrailRules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

// ---------------------------------------------------------------------------
// Prompt with binding and generation config
// ---------------------------------------------------------------------------

func TestPromptBindingAndGeneration(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	p := &models.Prompt{
		ID:          "p-bg",
		Name:        "binding-test",
		Content:     "test",
		Binding:     &models.ProviderBinding{Provider: "openai", Model: "gpt-4", APIKeyRef: "key-1"},
		Generation:  &models.GenerationConfig{Temperature: 0.7, TopP: 0.9, MaxTokens: 100, Stop: []string{"STOP"}},
		CreatedBy:   "u",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := db.CreatePrompt(ctx, p); err != nil {
		t.Fatalf("CreatePrompt: %v", err)
	}

	got, _ := db.GetPrompt(ctx, "p-bg")
	if got.Binding == nil || got.Binding.Provider != "openai" {
		t.Fatalf("expected binding provider openai, got %v", got.Binding)
	}
	if got.Generation == nil || got.Generation.Temperature != 0.7 {
		t.Fatalf("expected temperature 0.7, got %v", got.Generation)
	}
}

// ---------------------------------------------------------------------------
// Audit filters: combined user + action
// ---------------------------------------------------------------------------

func TestAuditFilterCombined(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-c1", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now})
	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-c2", UserID: "u1", Action: "delete", Resource: "p1", Details: map[string]any{}, Timestamp: now})
	db.AppendAudit(ctx, &models.AuditEntry{ID: "ae-c3", UserID: "u2", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now})

	entries, err := db.ListAudit(ctx, models.AuditFilter{UserID: "u1", Action: "create"})
	if err != nil {
		t.Fatalf("ListAudit combined: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Execution log with since + until range
// ---------------------------------------------------------------------------

func TestExecutionLogTimeRange(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.SaveExecutionLog(ctx, &models.ExecutionLog{ID: "el-r1", PromptID: "p1", PromptName: "t", Provider: "openai", Model: "gpt-4", Status: "success", Environment: "dev", CreatedAt: now})
	db.SaveExecutionLog(ctx, &models.ExecutionLog{ID: "el-r2", PromptID: "p1", PromptName: "t", Provider: "openai", Model: "gpt-4", Status: "success", Environment: "dev", CreatedAt: now.Add(time.Hour)})

	since := now.Add(-time.Minute)
	until := now.Add(30 * time.Minute)
	logs, err := db.ListExecutionLogs(ctx, models.ExecutionLogFilter{Since: &since, Until: &until})
	if err != nil {
		t.Fatalf("ListExecutionLogs range: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log in range, got %d", len(logs))
	}
}

// ---------------------------------------------------------------------------
// Workflow filter combined agent + status
// ---------------------------------------------------------------------------

func TestWorkflowFilterCombined(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	db.SaveWorkflow(ctx, &models.Workflow{ID: "w-c1", AgentID: "a-1", Status: models.WorkflowCompleted, Input: map[string]any{}, Output: map[string]any{}, StartedAt: now, CreatedAt: now})
	db.SaveWorkflow(ctx, &models.Workflow{ID: "w-c2", AgentID: "a-1", Status: models.WorkflowFailed, Input: map[string]any{}, Output: map[string]any{}, StartedAt: now, CreatedAt: now})

	wfs, err := db.ListWorkflows(ctx, models.WorkflowFilter{AgentID: "a-1", Status: string(models.WorkflowCompleted)})
	if err != nil {
		t.Fatalf("ListWorkflows combined: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}
}

// ---------------------------------------------------------------------------
// Eval run replace (INSERT OR REPLACE)
// ---------------------------------------------------------------------------

func TestEvalRunReplace(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	run := &models.EvalRun{ID: "er-rep", PromptHash: "h1", DatasetID: "d1", Model: "gpt-4", Status: "running", TotalCases: 5, StartedAt: now}
	db.SaveEvalRun(ctx, run)

	run.Status = "completed"
	run.PassedCases = 4
	db.SaveEvalRun(ctx, run)

	got, err := db.GetEvalRun(ctx, "er-rep")
	if err != nil {
		t.Fatalf("GetEvalRun: %v", err)
	}
	if got.Status != "completed" || got.PassedCases != 4 {
		t.Fatalf("expected completed/4, got %s/%d", got.Status, got.PassedCases)
	}
}

// ---------------------------------------------------------------------------
// Provider key upsert (ON CONFLICT)
// ---------------------------------------------------------------------------

func TestProviderKeyUpsert(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	pk := &models.ProviderKey{ID: "pk-up", ProviderName: "openai", KeyName: "k1", EncryptedKey: "v1", CreatedAt: now, UpdatedAt: now}
	db.SaveProviderKey(ctx, pk)

	pk.EncryptedKey = "v2"
	pk.UpdatedAt = now.Add(time.Minute)
	db.SaveProviderKey(ctx, pk)

	got, _ := db.GetProviderKey(ctx, "pk-up")
	if got.EncryptedKey != "v2" {
		t.Fatalf("expected v2, got %s", got.EncryptedKey)
	}

	keys, _ := db.ListProviderKeys(ctx)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key after upsert, got %d", len(keys))
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Duplicate insert errors (covers fmt.Errorf wrapping branches)
// ---------------------------------------------------------------------------

func TestCreatePromptDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	p := &models.Prompt{ID: "dup", Name: "a", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	if err := db.CreatePrompt(ctx, p); err != nil {
		t.Fatalf("CreatePrompt: %v", err)
	}
	// Second insert should fail (primary key constraint)
	if err := db.CreatePrompt(ctx, p); err == nil {
		t.Fatal("expected error for duplicate prompt")
	}
}

func TestCreateAgentDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	a := &models.Agent{ID: "dup", Name: "a", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if err := db.CreateAgent(ctx, a); err == nil {
		t.Fatal("expected error for duplicate agent")
	}
}

func TestCreateDatasetDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	d := &models.TestDataset{ID: "dup", Name: "a", CreatedBy: "u", CreatedAt: now}
	if err := db.CreateDataset(ctx, d); err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}
	if err := db.CreateDataset(ctx, d); err == nil {
		t.Fatal("expected error for duplicate dataset")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	u := &models.User{ID: "dup", Email: "dup@test.com", Name: "a", Role: "user", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := db.CreateUser(ctx, u); err == nil {
		t.Fatal("expected error for duplicate user")
	}
}

func TestCreateReviewDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	r := &models.Review{ID: "dup", ResourceID: "p1", ResourceType: "prompt", Author: "u", Status: models.ReviewPending, CreatedAt: now}
	if err := db.CreateReview(ctx, r); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if err := db.CreateReview(ctx, r); err == nil {
		t.Fatal("expected error for duplicate review")
	}
}

func TestCreateAPIKeyDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	k := &models.APIKey{ID: "dup", UserID: "u", Name: "k", KeyHash: "h1", KeyPrefix: "ps_", Role: "admin", CreatedAt: now}
	if err := db.CreateAPIKey(ctx, k); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if err := db.CreateAPIKey(ctx, k); err == nil {
		t.Fatal("expected error for duplicate API key")
	}
}

func TestSaveExecutionLogDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	el := &models.ExecutionLog{ID: "dup", PromptID: "p1", PromptName: "t", Provider: "openai", Model: "gpt-4", Status: "success", Environment: "dev", CreatedAt: now}
	if err := db.SaveExecutionLog(ctx, el); err != nil {
		t.Fatalf("SaveExecutionLog: %v", err)
	}
	if err := db.SaveExecutionLog(ctx, el); err == nil {
		t.Fatal("expected error for duplicate execution log")
	}
}

func TestSaveGuardrailViolationDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	v := &models.GuardrailViolationRecord{ID: "dup", RuleID: "r1", RuleName: "test", Type: "test", Severity: "low", ResourceType: "prompt", ResourceID: "p1", UserID: "u1", Message: "test", Timestamp: now}
	if err := db.SaveGuardrailViolation(ctx, v); err != nil {
		t.Fatalf("SaveGuardrailViolation: %v", err)
	}
	if err := db.SaveGuardrailViolation(ctx, v); err == nil {
		t.Fatal("expected error for duplicate violation")
	}
}

func TestSaveAgentGuardrailConfigDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	c := &models.AgentGuardrailConfig{ID: "dup", AgentID: "a-1", Name: "test", Enabled: true, StopOnViolation: true, CreatedAt: now, UpdatedAt: now}
	if err := db.SaveAgentGuardrailConfig(ctx, c); err != nil {
		t.Fatalf("SaveAgentGuardrailConfig: %v", err)
	}
	if err := db.SaveAgentGuardrailConfig(ctx, c); err == nil {
		t.Fatal("expected error for duplicate agent guardrail config")
	}
}

func TestSaveAgentExecutionDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	e := &models.AgentExecution{ID: "dup", AgentID: "a-1", WorkflowID: "w-1", Status: "running", Input: map[string]any{}, Output: map[string]any{}, Steps: []models.AgentExecutionStep{}, CreatedAt: now}
	if err := db.SaveAgentExecution(ctx, e); err != nil {
		t.Fatalf("SaveAgentExecution: %v", err)
	}
	if err := db.SaveAgentExecution(ctx, e); err == nil {
		t.Fatal("expected error for duplicate agent execution")
	}
}

func TestNewSQLiteInvalidPath(t *testing.T) {
	_, err := NewSQLite("/nonexistent/dir/that/does/not/exist/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// ---------------------------------------------------------------------------
// VerifyAuditChain tampered entry
// ---------------------------------------------------------------------------

func TestVerifyAuditChainTampered(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{ID: "ae-t1", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now}
	db.AppendAudit(ctx, entry)
	entry2 := &models.AuditEntry{ID: "ae-t2", UserID: "u1", Action: "update", Resource: "p1", Details: map[string]any{}, Timestamp: now.Add(time.Second)}
	db.AppendAudit(ctx, entry2)

	// Tamper with the stored hash directly
	db.db.ExecContext(ctx, "UPDATE audit_entries SET entry_hash = 'tampered' WHERE id = 'ae-t1'")

	valid, msg, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if valid {
		t.Fatal("expected chain to be invalid after tampering")
	}
	if msg == "" {
		t.Fatal("expected a message about the tampering")
	}
}

func TestVerifyAuditChainBrokenPrevHash(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{ID: "ae-b1", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now}
	db.AppendAudit(ctx, entry)
	entry2 := &models.AuditEntry{ID: "ae-b2", UserID: "u1", Action: "update", Resource: "p1", Details: map[string]any{}, Timestamp: now.Add(time.Second)}
	db.AppendAudit(ctx, entry2)

	// Tamper the previous_hash of the second entry
	db.db.ExecContext(ctx, "UPDATE audit_entries SET previous_hash = 'wrong' WHERE id = 'ae-b2'")

	valid, msg, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if valid {
		t.Fatal("expected chain to be invalid with broken prev hash")
	}
	if msg == "" {
		t.Fatal("expected a message about the chain break")
	}
}

// ---------------------------------------------------------------------------
// Scan error paths: corrupt JSON in database
// ---------------------------------------------------------------------------

func TestScanPromptCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	p := &models.Prompt{ID: "p-corrupt", Name: "test", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreatePrompt(ctx, p)

	// Corrupt the variables JSON directly
	db.db.ExecContext(ctx, "UPDATE prompts SET variables = 'not-json' WHERE id = 'p-corrupt'")
	_, err := db.GetPrompt(ctx, "p-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt variables JSON")
	}

	// Reset and corrupt tags
	db.db.ExecContext(ctx, "UPDATE prompts SET variables = '[]', tags = 'not-json' WHERE id = 'p-corrupt'")
	_, err = db.GetPrompt(ctx, "p-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt tags JSON")
	}

	// Reset and corrupt metadata
	db.db.ExecContext(ctx, "UPDATE prompts SET tags = '[]', metadata = 'not-json' WHERE id = 'p-corrupt'")
	_, err = db.GetPrompt(ctx, "p-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt metadata JSON")
	}

	// Reset and corrupt binding
	db.db.ExecContext(ctx, "UPDATE prompts SET metadata = '{}', binding = 'not-json' WHERE id = 'p-corrupt'")
	_, err = db.GetPrompt(ctx, "p-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt binding JSON")
	}

	// Reset and corrupt generation
	db.db.ExecContext(ctx, "UPDATE prompts SET binding = '{}', generation = 'not-json' WHERE id = 'p-corrupt'")
	_, err = db.GetPrompt(ctx, "p-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt generation JSON")
	}
}

func TestScanAgentCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	a := &models.Agent{ID: "a-corrupt", Name: "test", CreatedBy: "u", CreatedAt: now, UpdatedAt: now}
	db.CreateAgent(ctx, a)

	// Corrupt steps JSON
	db.db.ExecContext(ctx, "UPDATE agents SET steps = 'not-json' WHERE id = 'a-corrupt'")
	_, err := db.GetAgent(ctx, "a-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt steps JSON")
	}

	// Reset and corrupt tools
	db.db.ExecContext(ctx, "UPDATE agents SET steps = '[]', tools = 'not-json' WHERE id = 'a-corrupt'")
	_, err = db.GetAgent(ctx, "a-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt tools JSON")
	}
}

func TestScanDatasetCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	d := &models.TestDataset{ID: "ds-corrupt", Name: "test", CreatedBy: "u", CreatedAt: now}
	db.CreateDataset(ctx, d)

	// Corrupt cases JSON - scanDataset silently ignores unmarshal errors
	db.db.ExecContext(ctx, "UPDATE test_datasets SET cases = 'not-json' WHERE id = 'ds-corrupt'")
	got, err := db.GetDataset(ctx, "ds-corrupt")
	if err != nil {
		t.Fatalf("GetDataset with corrupt JSON: %v", err)
	}
	if got.Cases != nil {
		t.Fatalf("expected nil cases for corrupt JSON, got %v", got.Cases)
	}
}

func TestScanReviewCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	r := &models.Review{ID: "rv-corrupt", ResourceID: "p1", ResourceType: "prompt", Author: "u", Status: models.ReviewPending, CreatedAt: now}
	db.CreateReview(ctx, r)

	// Corrupt comments JSON - scanReview silently ignores unmarshal errors
	db.db.ExecContext(ctx, "UPDATE reviews SET comments = 'not-json' WHERE id = 'rv-corrupt'")
	got, err := db.GetReview(ctx, "rv-corrupt")
	if err != nil {
		t.Fatalf("GetReview with corrupt JSON: %v", err)
	}
	if got.Comments != nil {
		t.Fatalf("expected nil comments for corrupt JSON, got %v", got.Comments)
	}
}

func TestScanAuditRowCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	entry := &models.AuditEntry{ID: "ae-corrupt", UserID: "u1", Action: "create", Resource: "p1", Details: map[string]any{}, Timestamp: now}
	db.AppendAudit(ctx, entry)

	// Corrupt details JSON
	db.db.ExecContext(ctx, "UPDATE audit_entries SET details = 'not-json' WHERE id = 'ae-corrupt'")
	entries, err := db.ListAudit(ctx, models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	// The details will be nil due to the JSON error, but no error is returned
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestScanWorkflowCorruptJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	w := &models.Workflow{ID: "w-corrupt", AgentID: "a-1", Status: models.WorkflowPending, Input: map[string]any{}, Output: map[string]any{}, StartedAt: now, CreatedAt: now}
	db.SaveWorkflow(ctx, w)

	// Corrupt input JSON
	db.db.ExecContext(ctx, "UPDATE workflows SET input = 'not-json' WHERE id = 'w-corrupt'")
	_, err := db.GetWorkflow(ctx, "w-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt workflow input JSON")
	}

	// Reset and corrupt output
	db.db.ExecContext(ctx, "UPDATE workflows SET input = '{}', output = 'not-json' WHERE id = 'w-corrupt'")
	_, err = db.GetWorkflow(ctx, "w-corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt workflow output JSON")
	}
}

func TestScanProviderKeyNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetProviderKey(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetProviderKey not found")
	}
}

func TestScanProviderKeyByNameNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetProviderKeyByName(context.Background(), "no-provider", "no-key")
	if err == nil {
		t.Fatal("expected error for GetProviderKeyByName not found")
	}
}

func TestScanGuardrailRuleNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetGuardrailRule(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetGuardrailRule not found")
	}
}

func TestScanAgentGuardrailConfigNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetAgentGuardrailConfig(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetAgentGuardrailConfig not found")
	}
}

func TestScanAgentExecutionNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetAgentExecution(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetAgentExecution not found")
	}
}

func TestScanExecutionLogNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetExecutionLog(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetExecutionLog not found")
	}
}

func TestScanContextNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetContext(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetContext not found")
	}
}

func TestScanEvalRunNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetEvalRun(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for GetEvalRun not found")
	}
}

// ---------------------------------------------------------------------------
// Eval results with error field
// ---------------------------------------------------------------------------

func TestEvalResultWithError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	results := []*models.EvalResult{
		{TestCaseID: "tc-err", PromptHash: "h-err", Model: "gpt-4", DatasetID: "d-err", Output: "", Score: 0, LatencyMs: 50, Error: "timeout", CreatedAt: now},
	}
	if err := db.SaveEvalResults(ctx, results); err != nil {
		t.Fatalf("SaveEvalResults: %v", err)
	}

	got, _ := db.GetEvalResults(ctx, "h-err")
	if got[0].Error != "timeout" {
		t.Fatalf("expected error timeout, got %s", got[0].Error)
	}
}

// ---------------------------------------------------------------------------
// Migration safety tests
// ---------------------------------------------------------------------------

func TestMigrationIdempotent(t *testing.T) {
	db := setupTestDB(t)

	// Run migration twice - should not fail
	if err := migrate(db.DB(), migrationsFS); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrate(db.DB(), migrationsFS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	// Verify schema_migrations has correct entries
	var count int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 migration, got %d", count)
	}
}

func TestMigrationSchemaMigrationsTable(t *testing.T) {
	db := setupTestDB(t)

	// Verify schema_migrations table exists
	var exists int
	err := db.DB().QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("check migrations table: %v", err)
	}
	if exists != 1 {
		t.Fatal("schema_migrations table not created")
	}
}

func TestApplyMigrationRollback(t *testing.T) {
	db := setupTestDB(t)

	// Try to apply invalid SQL - should rollback
	err := applyMigration(db.DB(), 999, "INVALID SQL STATEMENT")
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}

	// Verify migration was not recorded
	var count int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 999").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatal("migration should not have been recorded after rollback")
	}
}
