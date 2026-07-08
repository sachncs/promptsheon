package eval

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/llm"
)

func TestRunVersion_Basic(t *testing.T) {
	r := NewRunner(&mockProvider{})
	ctx := context.Background()

	version := &capability.Version{
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
	_, err := r.RunVersion(context.Background(), &capability.Version{}, nil)
	if err == nil {
		t.Fatal("expected error for nil suite")
	}
}

func TestBuildVersionPrompt(t *testing.T) {
	r := NewRunner(&mockProvider{})

	version := &capability.Version{
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

	version := &capability.Version{
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

func (m *mockProvider) Complete(_ context.Context, _ *llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: "mock response",
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

func (m *mockProvider) Name() string { return "mock" }

func TestRunVersion_ThresholdNotMet(t *testing.T) {
	r := NewRunner(&mockProvider{})
	version := &capability.Version{ID: "ver-1"}
	suite := &capability.EvaluationSuite{
		Thresholds: map[string]float64{"accuracy": 0.99},
	}
	result, err := r.RunVersion(context.Background(), version, suite)
	if err != nil {
		t.Fatal(err)
	}
	if result.ThresholdsMet {
		t.Error("expected thresholds not met")
	}
}

func TestRunVersion_MultipleMetrics(t *testing.T) {
	r := NewRunner(&mockProvider{})
	version := &capability.Version{ID: "ver-multi"}
	suite := &capability.EvaluationSuite{
		Thresholds: map[string]float64{
			"precision":     0.90,
			"recall":        0.90,
			"hallucination": 0.01,
			"latency":       750.0,
			"cost":          0.008,
		},
	}
	result, err := r.RunVersion(context.Background(), version, suite)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ThresholdsMet {
		t.Error("expected all thresholds met")
	}
	if result.Precision != 0.93 {
		t.Errorf("precision = %f, want 0.93", result.Precision)
	}
	if result.Recall != 0.91 {
		t.Errorf("recall = %f, want 0.91", result.Recall)
	}
	if result.Hallucination != 0.03 {
		t.Errorf("hallucination = %f, want 0.03", result.Hallucination)
	}
	if result.LatencyMs != 750.0 {
		t.Errorf("latency = %f, want 750.0", result.LatencyMs)
	}
	if result.CostUSD != 0.008 {
		t.Errorf("cost = %f, want 0.008", result.CostUSD)
	}
}

func TestRunVersion_OneMetricFailsAmongMany(t *testing.T) {
	r := NewRunner(&mockProvider{})
	version := &capability.Version{ID: "ver-partial"}
	suite := &capability.EvaluationSuite{
		Thresholds: map[string]float64{
			"accuracy":  0.80,
			"precision": 0.95,
		},
	}
	result, err := r.RunVersion(context.Background(), version, suite)
	if err != nil {
		t.Fatal(err)
	}
	if result.ThresholdsMet {
		t.Error("expected thresholds not met (precision fails)")
	}
}

func TestRunVersion_EmptyThresholds(t *testing.T) {
	r := NewRunner(&mockProvider{})
	version := &capability.Version{ID: "ver-empty"}
	suite := &capability.EvaluationSuite{
		Thresholds: map[string]float64{},
	}
	result, err := r.RunVersion(context.Background(), version, suite)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ThresholdsMet {
		t.Error("expected thresholds met (empty map)")
	}
}
