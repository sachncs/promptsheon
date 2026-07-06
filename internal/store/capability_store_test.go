package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sachn-cs/promptsheon/internal/capability"
)

func TestCapabilityStore_WorkspaceCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	w := &capability.Workspace{
		ID:           "ws-1",
		Name:         "Acme Corp",
		Organization: "Acme Corporation",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.CreateWorkspace(ctx, w); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	got, err := db.GetWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("got name %q, want %q", got.Name, "Acme Corp")
	}

	list, err := db.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(list))
	}

	w.Name = "Acme Updated"
	w.UpdatedAt = time.Now().UTC()
	if err := db.UpdateWorkspace(ctx, w); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}

	got, _ = db.GetWorkspace(ctx, "ws-1")
	if got.Name != "Acme Updated" {
		t.Errorf("after update: got name %q", got.Name)
	}

	if err := db.DeleteWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}
	_, err = db.GetWorkspace(ctx, "ws-1")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCapabilityStore_ProjectCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create workspace first
	db.CreateWorkspace(ctx, &capability.Workspace{
		ID: "ws-1", Name: "Test", CreatedAt: now, UpdatedAt: now,
	})

	p := &capability.Project{
		ID: "proj-1", WorkspaceID: "ws-1", Name: "Customer Support",
		Description: "Support agent", CreatedAt: now, UpdatedAt: now,
	}

	if err := db.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := db.GetProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.WorkspaceID != "ws-1" {
		t.Errorf("workspace id mismatch")
	}

	list, err := db.ListProjects(ctx, "ws-1")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 project, got %d", len(list))
	}

	if err := db.DeleteProject(ctx, "proj-1"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	_, err = db.GetProject(ctx, "proj-1")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCapabilityStore_CapabilityCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupWorkspaceAndProject(t, db, now)

	c := &capability.Capability{
		ID: "cap-1", ProjectID: "proj-1", Name: "Summarize Invoice",
		Description: "Extract invoice data", Owner: "alice",
		Tags: []string{"finance", "invoices"}, State: capability.CapabilityStateDraft,
		CreatedAt: now, UpdatedAt: now,
	}

	if err := db.CreateCapability(ctx, c); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}

	got, err := db.GetCapability(ctx, "cap-1")
	if err != nil {
		t.Fatalf("GetCapability: %v", err)
	}
	if got.Name != "Summarize Invoice" {
		t.Errorf("name mismatch")
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
	if got.State != capability.CapabilityStateDraft {
		t.Errorf("expected draft state")
	}

	list, err := db.ListCapabilities(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(list))
	}

	c.State = capability.CapabilityStateActive
	c.CurrentVersionID = "ver-1"
	if err := db.UpdateCapability(ctx, c); err != nil {
		t.Fatalf("UpdateCapability: %v", err)
	}
	got, _ = db.GetCapability(ctx, "cap-1")
	if got.State != capability.CapabilityStateActive {
		t.Errorf("expected active state after update")
	}
	if got.CurrentVersionID != "ver-1" {
		t.Errorf("expected current_version_id ver-1")
	}

	if err := db.DeleteCapability(ctx, "cap-1"); err != nil {
		t.Fatalf("DeleteCapability: %v", err)
	}
	_, err = db.GetCapability(ctx, "cap-1")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCapabilityStore_VersionCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)

	v := &capability.CapabilityVersion{
		ID:           "ver-1",
		CapabilityID: "cap-1",
		Version:      1,
		Prompt: capability.Prompt{
			Instructions: "Summarize this invoice",
			Variables: []capability.PromptVariable{
				{Name: "invoice_id", Type: "string", Required: true},
			},
		},
		ModelPolicy: capability.ModelPolicy{
			Requirements: capability.ModelRequirements{
				NeedsJSON:  true,
				MaxLatencyMs: 2000,
			},
			SelectionStrategy: capability.SelectionStrategyCostOptimized,
		},
		RuntimePolicy: capability.RuntimePolicy{
			Temperature: 0.2,
			MaxTokens:   1000,
			Retries:     3,
		},
		CreatedAt: now,
		CreatedBy: "alice",
	}

	if err := db.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	got, err := db.GetVersion(ctx, "ver-1")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if got.Prompt.Instructions != "Summarize this invoice" {
		t.Errorf("prompt instructions mismatch")
	}
	if len(got.Prompt.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(got.Prompt.Variables))
	}
	if got.ModelPolicy.SelectionStrategy != capability.SelectionStrategyCostOptimized {
		t.Errorf("expected cost_optimized strategy")
	}
	if got.RuntimePolicy.Retries != 3 {
		t.Errorf("expected 3 retries, got %d", got.RuntimePolicy.Retries)
	}

	list, err := db.ListVersions(ctx, "cap-1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 version, got %d", len(list))
	}

	latest, err := db.GetLatestVersion(ctx, "cap-1")
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if latest.ID != "ver-1" {
		t.Errorf("expected latest version ver-1")
	}
}

func TestCapabilityStore_MultipleVersions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)

	for i := 1; i <= 3; i++ {
		v := &capability.CapabilityVersion{
			ID:           fmt.Sprintf("ver-%d", i),
			CapabilityID: "cap-1",
			Version:      i,
			Prompt: capability.Prompt{
				Instructions: fmt.Sprintf("Version %d instructions", i),
			},
			CreatedAt: now,
		}
		if err := db.CreateVersion(ctx, v); err != nil {
			t.Fatalf("CreateVersion v%d: %v", i, err)
		}
	}

	list, err := db.ListVersions(ctx, "cap-1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(list))
	}

	latest, _ := db.GetLatestVersion(ctx, "cap-1")
	if latest.Version != 3 {
		t.Errorf("expected latest version 3, got %d", latest.Version)
	}
}

func TestCapabilityStore_ExecutionCRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupVersion(t, db, now)

	e := &capability.Execution{
		ID:                  "exec-1",
		CapabilityVersionID: "ver-1",
		Timestamp:           now,
		Inputs:              map[string]any{"text": "invoice-123"},
		Outputs:             map[string]any{"total": "$100.00"},
		Model:               "gpt-4",
		Provider:            "openai",
		LatencyMs:           1200,
		CostUSD:             0.015,
		PromptTokens:        500,
		CompletionTokens:    100,
		TotalTokens:         600,
		Environment:         "prod",
	}

	if err := db.CreateExecution(ctx, e); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}

	got, err := db.GetExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if got.LatencyMs != 1200 {
		t.Errorf("expected 1200ms latency, got %d", got.LatencyMs)
	}
	if got.TotalTokens != 600 {
		t.Errorf("expected 600 tokens, got %d", got.TotalTokens)
	}
	if got.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", got.Model)
	}

	// Test with version filter
	filter := ExecutionFilter{
		CapabilityVersionID: "ver-1",
		Limit:               10,
	}
	list, err := db.ListExecutions(ctx, filter)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(list))
	}

	// Test with inputs/outputs JSON serialization
	if got.Inputs["text"] != "invoice-123" {
		t.Errorf("input mismatch")
	}
	if got.Outputs["total"] != "$100.00" {
		t.Errorf("output mismatch")
	}
}

func TestCapabilityStore_VersionWithFullConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)

	v := &capability.CapabilityVersion{
		ID:           "ver-full",
		CapabilityID: "cap-1",
		Version:      1,
		Prompt: capability.Prompt{
			Role:         "assistant",
			Instructions: "Help the user with their query",
			Examples: []capability.PromptExample{
				{Input: "Q: What is X?", Output: "A: X is Y"},
			},
			Variables: []capability.PromptVariable{
				{Name: "query", Type: "string", Required: true},
			},
			Template: "User query: {{.query}}",
			LocaleVariants: map[string]string{
				"fr": "Requête: {{.query}}",
			},
		},
		ModelPolicy: capability.ModelPolicy{
			Requirements: capability.ModelRequirements{
				NeedsReasoning: true,
				NeedsToolUse:   true,
				MaxLatencyMs:   5000,
				MaxCostUSD:     0.05,
			},
			SelectionStrategy: capability.SelectionStrategyQualityOptimized,
			ProviderHints:    []string{"openai", "anthropic"},
		},
		ContextContract: capability.ContextContract{
			RequiredContext:    []capability.ContextRef{{Key: "user", Source: "session"}},
			MaximumSize:        8192,
			RetrievalStrategy:  "hybrid",
		},
		Knowledge: []capability.KnowledgeSource{
			{ID: "ks-1", Name: "docs", Type: "rag", Version: "v2", EmbeddingModel: "text-embedding-3-small"},
		},
		Memory: capability.MemoryConfig{
			SessionMemory: true, ConversationMemory: true, MaxSessionTokens: 4096,
		},
		Guardrails: []capability.Guardrail{
			{ID: "gr-1", Name: "pii", Phase: capability.GuardrailPhasePre, Severity: "high"},
		},
		Tools: []capability.Tool{
			{ID: "t-1", Name: "search", Version: "1.0", Type: "http"},
		},
		MCPServers: []capability.MCPServer{
			{ID: "mcp-1", Name: "search-mcp", Transport: "sse"},
		},
		RuntimePolicy: capability.RuntimePolicy{
			Retries: 2, TimeoutMs: 30000, Caching: "semantic", Temperature: 0.3, MaxTokens: 2048,
		},
		EvaluationSuite: capability.EvaluationSuite{
			Metrics: []string{"accuracy", "hallucination"},
			Thresholds: map[string]float64{"accuracy": 0.9},
		},
		CreatedAt: now,
		CreatedBy: "bob",
	}

	if err := db.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion with full config: %v", err)
	}

	got, err := db.GetVersion(ctx, "ver-full")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}

	// Verify all fields round-trip correctly
	if got.Prompt.Role != "assistant" {
		t.Errorf("prompt role mismatch")
	}
	if len(got.Prompt.Examples) != 1 {
		t.Errorf("expected 1 example, got %d", len(got.Prompt.Examples))
	}
	if len(got.Prompt.LocaleVariants) != 1 {
		t.Errorf("expected 1 locale variant, got %d", len(got.Prompt.LocaleVariants))
	}
	if got.ModelPolicy.SelectionStrategy != capability.SelectionStrategyQualityOptimized {
		t.Errorf("selection strategy mismatch")
	}
	if len(got.ModelPolicy.ProviderHints) != 2 {
		t.Errorf("expected 2 provider hints")
	}
	if got.ContextContract.MaximumSize != 8192 {
		t.Errorf("context max size mismatch")
	}
	if len(got.Knowledge) != 1 {
		t.Errorf("expected 1 knowledge source")
	}
	if !got.Memory.SessionMemory {
		t.Errorf("expected session memory enabled")
	}
	if len(got.Guardrails) != 1 {
		t.Errorf("expected 1 guardrail")
	}
	if len(got.Tools) != 1 {
		t.Errorf("expected 1 tool")
	}
	if len(got.MCPServers) != 1 {
		t.Errorf("expected 1 mcp server")
	}
	if got.RuntimePolicy.Caching != "semantic" {
		t.Errorf("expected semantic caching")
	}
	if len(got.EvaluationSuite.Metrics) != 2 {
		t.Errorf("expected 2 eval metrics")
	}
	if got.CreatedBy != "bob" {
		t.Errorf("expected created_by bob")
	}
}

func TestCapabilityStore_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.GetWorkspace(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for workspace, got %v", err)
	}

	_, err = db.GetProject(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for project, got %v", err)
	}

	_, err = db.GetCapability(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for capability, got %v", err)
	}

	_, err = db.GetVersion(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for version, got %v", err)
	}

	_, err = db.GetExecution(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for execution, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupWorkspaceAndProject(t *testing.T, db *SQLite, now time.Time) {
	t.Helper()
	if err := db.CreateWorkspace(context.Background(), &capability.Workspace{
		ID: "ws-1", Name: "Test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("setup workspace: %v", err)
	}
	if err := db.CreateProject(context.Background(), &capability.Project{
		ID: "proj-1", WorkspaceID: "ws-1", Name: "Test Project", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("setup project: %v", err)
	}
}

func setupCapability(t *testing.T, db *SQLite, now time.Time) {
	t.Helper()
	setupWorkspaceAndProject(t, db, now)
	if err := db.CreateCapability(context.Background(), &capability.Capability{
		ID: "cap-1", ProjectID: "proj-1", Name: "Test Capability",
		State: capability.CapabilityStateDraft, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("setup capability: %v", err)
	}
}

func setupVersion(t *testing.T, db *SQLite, now time.Time) {
	t.Helper()
	setupCapability(t, db, now)
	if err := db.CreateVersion(context.Background(), &capability.CapabilityVersion{
		ID: "ver-1", CapabilityID: "cap-1", Version: 1,
		Prompt: capability.Prompt{Instructions: "test"},
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("setup version: %v", err)
	}
}
