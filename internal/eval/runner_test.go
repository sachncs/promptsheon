package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/llm"
)

func TestNewRunnerStoresProvider(t *testing.T) {
	t.Parallel()
	mock := llm.NewMock("ok")
	r := NewRunner(mock)
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
}

func TestNewRunnerNilProvider(t *testing.T) {
	t.Parallel()
	r := NewRunner(nil)
	if r == nil {
		t.Fatal("NewRunner(nil) returned nil")
	}
}

func goodHash(t *testing.T) string {
	t.Helper()
	return strings.Repeat("a", 64)
}

func TestRunnerEvaluateExactMatch(t *testing.T) {
	t.Parallel()
	mock := llm.NewMock("42")
	r := NewRunner(mock)
	version := &capability.Version{
		ID:           "v1",
		CapabilityID: "c1",
		Version:      1,
		Manifest: capability.Manifest{
			Prompt: capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: goodHash(t)},
		},
	}
	ds := Dataset{
		Name: "smoke",
		Examples: []Example{
			{Name: "q1", Inputs: map[string]string{}, Expected: "42"},
		},
	}
	res, err := r.Evaluate(context.Background(), version, ds)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.TotalExamples != 1 || res.Passed != 1 {
		t.Errorf("result = %+v, want 1/1 passed", res)
	}
	if res.AverageScore != 1.0 {
		t.Errorf("AverageScore = %f, want 1.0", res.AverageScore)
	}
}

func TestRunnerEvaluateContainsMatch(t *testing.T) {
	t.Parallel()
	mock := llm.NewMock("The answer is 42 and that is it.")
	r := NewRunner(mock)
	r.SetScorer(ContainsMatch{})
	version := &capability.Version{
		ID:           "v1",
		CapabilityID: "c1",
		Version:      1,
		Manifest: capability.Manifest{
			Prompt: capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: goodHash(t)},
		},
	}
	ds := Dataset{
		Name: "smoke",
		Examples: []Example{
			{Name: "q1", Inputs: map[string]string{}, Expected: "42"},
			{Name: "q2", Inputs: map[string]string{}, Expected: "absent"},
		},
	}
	res, err := r.Evaluate(context.Background(), version, ds)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Passed != 1 || res.Failed != 1 {
		t.Errorf("result = %+v, want 1 passed + 1 failed", res)
	}
}

func TestRunnerEvaluateProviderError(t *testing.T) {
	t.Parallel()
	mock := &llm.Mock{Error: context.DeadlineExceeded}
	r := NewRunner(mock)
	version := &capability.Version{
		ID:           "v1",
		CapabilityID: "c1",
		Version:      1,
		Manifest: capability.Manifest{
			Prompt: capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: goodHash(t)},
		},
	}
	ds := Dataset{
		Name:     "smoke",
		Examples: []Example{{Name: "q1", Expected: "x"}},
	}
	res, err := r.Evaluate(context.Background(), version, ds)
	if err != nil {
		t.Fatalf("Evaluate should swallow per-example errors: %v", err)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1", res.Failed)
	}
	if res.PerExample[0].Error == "" {
		t.Errorf("expected per-example error to be recorded")
	}
}

func TestRunnerEvaluateNilVersion(t *testing.T) {
	t.Parallel()
	r := NewRunner(llm.NewMock("x"))
	if _, err := r.Evaluate(context.Background(), nil, Dataset{}); err == nil {
		t.Error("expected error for nil version")
	}
}

func TestRunnerEvaluateNilProvider(t *testing.T) {
	t.Parallel()
	r := NewRunner(nil)
	_, err := r.Evaluate(context.Background(), &capability.Version{}, Dataset{})
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestResultJSONShape(t *testing.T) {
	t.Parallel()
	r := Result{
		DatasetName:   "smoke",
		TotalExamples: 2,
		Passed:        1,
		Failed:        1,
		AverageScore:  0.5,
	}
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"dataset_name":"smoke"`) {
		t.Errorf("expected dataset_name in JSON: %s", b)
	}
}

func TestSubstitute(t *testing.T) {
	// ponytail: simple test that the substitution helper produces
	// the expected output for a known template.
	if got := substitute("hello {{name}}", map[string]string{"name": "world"}); got != "hello world" {
		t.Errorf("substitute = %q, want %q", got, "hello world")
	}
}
