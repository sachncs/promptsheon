package eval

import (
	"context"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/capability"
	"github.com/sachn-cs/promptsheon/internal/llm"
)

func TestRunVersion_Basic(t *testing.T) {
	r := NewRunner(&mockProvider{})
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "Summarize: {{.text}}",
		},
	}

	suite := &capability.EvaluationSuite{
		Datasets: []capability.EvalDatasetRef{
			{ID: "ds-1", Name: "test-set"},
		},
		Metrics: []string{"accuracy", "latency"},
		Thresholds: map[string]float64{
			"accuracy": 0.8,
		},
	}

	result, err := r.RunVersion(ctx, version, suite)
	if err != nil {
		t.Fatalf("RunVersion: %v", err)
	}
	if result.Accuracy != 0.95 {
		t.Errorf("expected accuracy 0.95, got %f", result.Accuracy)
	}
	if !result.ThresholdsMet {
		t.Errorf("expected thresholds met")
	}
	if result.CapabilityVersionID != "ver-1" {
		t.Errorf("expected version ID ver-1")
	}
}

func TestRunVersion_NilVersion(t *testing.T) {
	r := NewRunner(&mockProvider{})
	_, err := r.RunVersion(context.Background(), nil, &capability.EvaluationSuite{})
	if err == nil {
		t.Fatal("expected error for nil version")
	}
}

func TestRunVersion_NilSuite(t *testing.T) {
	r := NewRunner(&mockProvider{})
	_, err := r.RunVersion(context.Background(), &capability.CapabilityVersion{}, nil)
	if err == nil {
		t.Fatal("expected error for nil suite")
	}
}

func TestBuildVersionPrompt(t *testing.T) {
	r := NewRunner(&mockProvider{})

	version := &capability.CapabilityVersion{
		Prompt: capability.Prompt{
			Instructions: "Process {{.id}} and {{.name}}",
		},
	}

	input := map[string]any{
		"id":   "123",
		"name": "test",
	}

	result := r.buildVersionPrompt(version, input)
	expected := "Process 123 and test"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildVersionPromptWithTemplate(t *testing.T) {
	r := NewRunner(&mockProvider{})

	version := &capability.CapabilityVersion{
		Prompt: capability.Prompt{
			Instructions: "fallback",
			Template:     "{{.key}}",
		},
	}

	result := r.buildVersionPrompt(version, map[string]any{"key": "value"})
	if result != "value" {
		t.Errorf("expected 'value', got %q", result)
	}
}

// mockProvider is a minimal LLM provider for testing.
type mockProvider struct{}

func (m *mockProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: "mock response",
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

func (m *mockProvider) Name() string { return "mock" }
