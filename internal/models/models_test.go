package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPromptJSON(t *testing.T) {
	p := Prompt{
		ID:          "p-001",
		Name:        "test-prompt",
		Description: "A test prompt",
		Content:     "You are a helpful assistant.",
		Variables: []Variable{
			{Name: "topic", Type: "string", Required: true, Description: "The topic to discuss"},
		},
		Tags:      []string{"assistant", "general"},
		ModelHint: "gpt-4",
		Version:   1,
		Status:    StatusDraft,
		CASHash:   "abc123",
		CreatedBy: "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{"env": "test"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped Prompt
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.ID != p.ID {
		t.Fatalf("ID mismatch: %s vs %s", roundTripped.ID, p.ID)
	}
	if roundTripped.Content != p.Content {
		t.Fatalf("Content mismatch: %s vs %s", roundTripped.Content, p.Content)
	}
	if len(roundTripped.Variables) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(roundTripped.Variables))
	}
	if roundTripped.Variables[0].Name != "topic" {
		t.Fatalf("expected variable name 'topic', got %q", roundTripped.Variables[0].Name)
	}
}

func TestAgentJSON(t *testing.T) {
	a := Agent{
		ID:          "a-001",
		Name:        "research-agent",
		Description: "Multi-step research agent",
		Steps: []AgentStep{
			{
				ID:         "step-1",
				PromptID:   "p-001",
				PromptHash: "hash1",
				DependsOn:  nil,
				OutputKey:  "research",
			},
			{
				ID:         "step-2",
				PromptID:   "p-002",
				PromptHash: "hash2",
				DependsOn:  []string{"step-1"},
				OutputKey:  "summary",
			},
		},
		Tools: []ToolRef{
			{Name: "web_search", Type: ToolHTTP, Config: map[string]any{"url": "https://api.search.com"}},
		},
		Status:    StatusDraft,
		CreatedBy: "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped Agent
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(roundTripped.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(roundTripped.Steps))
	}
	if roundTripped.Steps[1].DependsOn[0] != "step-1" {
		t.Fatalf("expected dependency on step-1, got %q", roundTripped.Steps[1].DependsOn[0])
	}
	if len(roundTripped.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(roundTripped.Tools))
	}
}

func TestTestDatasetJSON(t *testing.T) {
	d := TestDataset{
		ID:   "ds-001",
		Name: "basic-qa",
		Cases: []TestCase{
			{
				ID:               "tc-001",
				Input:            map[string]any{"question": "What is Go?"},
				ExpectedOutput:   "Go is a programming language",
				ExpectedContains: []string{"programming", "language"},
				Tags:             []string{"basic"},
			},
		},
		CreatedBy: "user-1",
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped TestDataset
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(roundTripped.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(roundTripped.Cases))
	}
	if roundTripped.Cases[0].Input["question"] != "What is Go?" {
		t.Fatalf("unexpected input: %v", roundTripped.Cases[0].Input)
	}
}

func TestEvalResultJSON(t *testing.T) {
	r := EvalResult{
		ID:         "ev-001",
		TestCaseID: "tc-001",
		PromptHash: "abc123",
		Model:      "gpt-4",
		Output:     "Go is a statically typed programming language",
		Score:      0.95,
		LatencyMs:  250,
		TokenUsage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		HallucinationScore: 0.02,
		Passed:             true,
		CreatedAt:          time.Now(),
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped EvalResult
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.Score != 0.95 {
		t.Fatalf("expected score 0.95, got %f", roundTripped.Score)
	}
	if roundTripped.TokenUsage.TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens, got %d", roundTripped.TokenUsage.TotalTokens)
	}
}

func TestReviewJSON(t *testing.T) {
	now := time.Now()
	r := Review{
		ID:           "rv-001",
		ResourceID:   "p-001",
		ResourceType: "prompt",
		Author:       "user-1",
		Status:       ReviewPending,
		Comments: []Comment{
			{ID: "c-001", UserID: "user-2", Content: "Looks good", CreatedAt: now},
		},
		CreatedAt:  now,
		ResolvedAt: nil,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped Review
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.Status != ReviewPending {
		t.Fatalf("expected pending status, got %q", roundTripped.Status)
	}
	if len(roundTripped.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(roundTripped.Comments))
	}
}

func TestAuditEntryJSON(t *testing.T) {
	e := AuditEntry{
		ID:       "ae-001",
		UserID:   "user-1",
		Action:   "create",
		Resource: "prompt:p-001",
		Details:  map[string]any{"name": "test-prompt"},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped AuditEntry
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.Action != "create" {
		t.Fatalf("expected action 'create', got %q", roundTripped.Action)
	}
	if roundTripped.Resource != "prompt:p-001" {
		t.Fatalf("expected resource 'prompt:p-001', got %q", roundTripped.Resource)
	}
}

func TestPromptStatusConstants(t *testing.T) {
	if StatusDraft != "draft" {
		t.Fatalf("expected 'draft', got %q", StatusDraft)
	}
	if StatusApproved != "approved" {
		t.Fatalf("expected 'approved', got %q", StatusApproved)
	}
	if StatusArchived != "archived" {
		t.Fatalf("expected 'archived', got %q", StatusArchived)
	}
}

func TestToolTypeConstants(t *testing.T) {
	if ToolHTTP != "http" {
		t.Fatalf("expected 'http', got %q", ToolHTTP)
	}
	if ToolShell != "shell" {
		t.Fatalf("expected 'shell', got %q", ToolShell)
	}
	if ToolJSON != "json_transform" {
		t.Fatalf("expected 'json_transform', got %q", ToolJSON)
	}
	if ToolPrompt != "prompt_call" {
		t.Fatalf("expected 'prompt_call', got %q", ToolPrompt)
	}
}

func TestReviewStatusConstants(t *testing.T) {
	if ReviewPending != "pending" {
		t.Fatalf("expected 'pending', got %q", ReviewPending)
	}
	if ReviewApproved != "approved" {
		t.Fatalf("expected 'approved', got %q", ReviewApproved)
	}
	if ReviewRejected != "rejected" {
		t.Fatalf("expected 'rejected', got %q", ReviewRejected)
	}
}

func TestAPIKeyJSON(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	k := APIKey{
		ID:        "ak-001",
		UserID:    "user-1",
		Name:      "production-key",
		KeyHash:   "abc123hash",
		KeyPrefix: "ps_abc",
		Role:      "admin",
		ExpiresAt: &expires,
		CreatedAt: now,
		Revoked:   false,
	}

	data, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped APIKey
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.ID != "ak-001" {
		t.Fatalf("expected ID 'ak-001', got %q", roundTripped.ID)
	}
	if roundTripped.KeyHash != "" {
		t.Fatalf("expected key_hash to be omitted from JSON, got %q", roundTripped.KeyHash)
	}
	if roundTripped.KeyPrefix != "ps_abc" {
		t.Fatalf("expected key_prefix 'ps_abc', got %q", roundTripped.KeyPrefix)
	}
}
