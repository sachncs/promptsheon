package selfevolve

import (
	"context"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/eval"
	"github.com/sachncs/promptsheon/internal/harness"
)

func validatorCases() []harness.DatasetCase {
	return []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: []byte(`{"q":"ping"}`), Expected: []byte(`"pong"`)},
		{ID: "k1", DatasetID: "ds1", Seq: 1, Inputs: []byte(`{"q":"hi"}`), Expected: []byte(`"pong"`)},
		{ID: "k2", DatasetID: "ds1", Seq: 2, Inputs: []byte(`{"q":"yo"}`), Expected: []byte(`"pong"`)},
	}
}

func validatorScorer() eval.Scorer { return eval.ScorerContains }

type testCaseLoader struct{ cases []harness.DatasetCase }

func (l *testCaseLoader) ListDatasetCases(_ context.Context, _ string) ([]harness.DatasetCase, error) {
	return l.cases, nil
}

func newCaseLoader(c []harness.DatasetCase) CaseLoader { return &testCaseLoader{cases: c} }

func TestHarnessValidator_AllPass(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) { return "pong", nil }
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("reply with pong"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 3 || run.Failed != 0 || run.Total != 3 {
		t.Errorf("counts = %d/%d/%d, want 3/0/3", run.Passed, run.Failed, run.Total)
	}
	if run.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", run.Score)
	}
	if run.Status != harness.RunPassed {
		t.Errorf("Status = %q, want RunPassed", run.Status)
	}
	if !strings.HasPrefix(run.ID, "self-evolve-validate-c1-") {
		t.Errorf("ID = %q, want prefix self-evolve-validate-c1-", run.ID)
	}
}

func TestHarnessValidator_AllFail(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) { return "wrong", nil }
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 0 || run.Failed != 3 {
		t.Errorf("counts = %d/%d, want 0/3", run.Passed, run.Failed)
	}
	if run.Score != 0 {
		t.Errorf("Score = %v, want 0", run.Score)
	}
}

func TestHarnessValidator_Mixed(t *testing.T) {
	invoke := func(_ context.Context, req LLMInvokeRequest) (string, error) {
		if strings.Contains(req.User, "ping") {
			return "pong", nil
		}
		return "wrong", nil
	}
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 1 || run.Failed != 2 {
		t.Errorf("counts = %d/%d, want 1/2", run.Passed, run.Failed)
	}
}

func TestHarnessValidator_InvokeErrorCountsAsFail(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) {
		return "", context.DeadlineExceeded
	}
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Failed != 3 || run.Passed != 0 {
		t.Errorf("counts = %d/%d, want 0/3", run.Passed, run.Failed)
	}
}

func TestHarnessValidator_InputsEmptyDefaultsToObject(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) { return "pong", nil }
	v := NewHarnessValidator(newCaseLoader([]harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: nil, Expected: []byte(`"pong"`)},
	}), invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 1 {
		t.Errorf("Passed = %d, want 1", run.Passed)
	}
}

func TestHarnessValidator_EmptyDataset(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) { return "x", nil }
	v := NewHarnessValidator(newCaseLoader(nil), invoke)
	if _, err := v.Validate(context.Background(), "c1", []byte("p"), "missing-ds"); err == nil {
		t.Fatalf("expected error on empty dataset")
	}
}

func TestHarnessValidator_UnknownScorer(t *testing.T) {
	invoke := func(_ context.Context, _ LLMInvokeRequest) (string, error) { return "x", nil }
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = "not_a_real_scorer"
	if _, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1"); err == nil {
		t.Fatalf("expected error on unknown scorer")
	}
}

func TestHarnessValidator_PassesSystemPrompt(t *testing.T) {
	var sawPrompt string
	invoke := func(_ context.Context, req LLMInvokeRequest) (string, error) {
		sawPrompt = req.System
		return "pong", nil
	}
	v := NewHarnessValidator(newCaseLoader(validatorCases()), invoke)
	v.Scorer = validatorScorer()
	if _, err := v.Validate(context.Background(), "c1", []byte("THE PROMPT"), "ds1"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sawPrompt != "THE PROMPT" {
		t.Errorf("System = %q, want THE PROMPT", sawPrompt)
	}
}
