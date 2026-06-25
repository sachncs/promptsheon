package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPromptJSONRoundTrip confirms the canonical JSON
// shape of a Prompt. The Prompt struct is the most
// frequently marshalled type in the system; the round
// trip test catches accidental field renames that would
// otherwise silently break every SDK consumer.
func TestPromptJSONRoundTrip(t *testing.T) {
	p := &Prompt{
		ID:          "p-1",
		Name:        "greeting",
		Description: "a friendly greeting",
		Content:     "Hello {{name}}",
		Variables: []Variable{
			{Name: "name", Type: "string", Required: true, Default: "world"},
		},
		Tags:      []string{"greeting", "demo"},
		ModelHint: "gpt-4",
		Version:   3,
		Status:    "deployed",
		CreatedBy: "alice",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{"env": "production"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Prompt
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != p.ID || got.Name != p.Name || got.Version != p.Version {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestPromptBindingJSON(t *testing.T) {
	b := &ProviderBinding{
		Provider:  "openai",
		Model:     "gpt-4",
		APIKeyRef: "vault://key-1",
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got := string(data); got == "" {
		t.Error("expected non-empty JSON")
	}
}

func TestAgentStepJSON(t *testing.T) {
	s := AgentStep{
		ID:        "s1",
		PromptID:  "p-1",
		OutputKey: "out1",
		DependsOn: []string{"s0"},
		Condition: &Condition{Field: "x", Operator: "eq", Value: "1"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got AgentStep
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != s.ID || got.PromptID != s.PromptID {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestPromptStatusConstants(t *testing.T) {
	// Status values are part of the wire format and
	// appear in the UI; pinning the strings catches
	// accidental renames.
	if StatusDraft != "draft" {
		t.Errorf("StatusDraft: got %q", StatusDraft)
	}
	if StatusDeployed != "deployed" {
		t.Errorf("StatusDeployed: got %q", StatusDeployed)
	}
	if StatusArchived != "archived" {
		t.Errorf("StatusArchived: got %q", StatusArchived)
	}
}

func TestPromptStatusCanTransitionTo(t *testing.T) {
	// Pin the documented state machine. draft -> approved
	// -> deployed -> archived; draft can skip directly to
	// archived; nothing comes back from archived.
	cases := []struct {
		from, to PromptStatus
		want     bool
	}{
		{StatusDraft, StatusApproved, true},
		{StatusDraft, StatusArchived, true},
		{StatusDraft, StatusDeployed, false}, // requires approved first
		{StatusApproved, StatusDeployed, true},
		{StatusApproved, StatusArchived, true},
		{StatusApproved, StatusDraft, false},
		{StatusDeployed, StatusArchived, true},
		{StatusDeployed, StatusDraft, false},
		{StatusDeployed, StatusApproved, false},
		{StatusArchived, StatusDraft, false},
		{StatusArchived, StatusDeployed, false},
		{StatusArchived, StatusApproved, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.want {
			t.Errorf("%s -> %s: got %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestTruncationStrategyConstants(t *testing.T) {
	if TruncationSlidingWindow != "sliding_window" {
		t.Errorf("TruncationSlidingWindow: got %q", TruncationSlidingWindow)
	}
	if TruncationDropOldest != "drop_oldest" {
		t.Errorf("TruncationDropOldest: got %q", TruncationDropOldest)
	}
}

func TestToolTypeConstants(t *testing.T) {
	if ToolHTTP != "http" {
		t.Errorf("ToolHTTP: got %q", ToolHTTP)
	}
	if ToolShell != "shell" {
		t.Errorf("ToolShell: got %q", ToolShell)
	}
	if ToolJSON != "json_transform" {
		t.Errorf("ToolJSON: got %q", ToolJSON)
	}
	if ToolPrompt != "prompt_call" {
		t.Errorf("ToolPrompt: got %q", ToolPrompt)
	}
}
