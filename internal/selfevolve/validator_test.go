package selfevolve

import (
	"context"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/eval"
	"github.com/sachncs/promptsheon/internal/harness"
)

// validatorCases returns 3 cases the validator can score.
func validatorCases() []harness.DatasetCase {
	return []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: []byte(`{"q":"ping"}`), Expected: []byte(`"pong"`)},
		{ID: "k1", DatasetID: "ds1", Seq: 1, Inputs: []byte(`{"q":"hi"}`), Expected: []byte(`"pong"`)},
		{ID: "k2", DatasetID: "ds1", Seq: 2, Inputs: []byte(`{"q":"yo"}`), Expected: []byte(`"pong"`)},
	}
}

func validatorScorer() eval.Scorer { return eval.ScorerContains }

func TestHarnessValidator_AllPass(t *testing.T) {
	loader := newCaseLoader(validatorCases())
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		return "pong", nil
	}
	v := NewHarnessValidator(loader, invoke)
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
	loader := newCaseLoader(validatorCases())
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		return "wrong", nil
	}
	v := NewHarnessValidator(loader, invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("prompt"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 0 || run.Failed != 3 {
		t.Errorf("counts = %d/%d, want 0/3", run.Passed, run.Failed)
	}
	if run.Score != 0 {
		t.Errorf("Score = %v, want 0", run.Score)
	}
	if run.Status != harness.RunFailed {
		t.Errorf("Status = %q, want RunFailed", run.Status)
	}
}

func TestHarnessValidator_Mixed(t *testing.T) {
	cases := validatorCases()
	loader := newCaseLoader(cases)
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		// Return "pong" only on the first call (Seq 0); other
		// cases get "wrong".
		if strings.Contains(req.User, "ping") {
			return "pong", nil
		}
		return "wrong", nil
	}
	v := NewHarnessValidator(loader, invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("prompt"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 1 || run.Failed != 2 {
		t.Errorf("counts = %d/%d, want 1/2", run.Passed, run.Failed)
	}
}

func TestHarnessValidator_InvokeErrorCountsAsFail(t *testing.T) {
	loader := newCaseLoader(validatorCases())
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		return "", context.DeadlineExceeded
	}
	v := NewHarnessValidator(loader, invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("prompt"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Failed != 3 || run.Passed != 0 {
		t.Errorf("counts = %d/%d, want 0/3", run.Passed, run.Failed)
	}
}

func TestHarnessValidator_InputsEmptyDefaultsToObject(t *testing.T) {
	loader := newCaseLoader([]harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: nil, Expected: []byte(`"pong"`)},
	})
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		// Empty inputs default to "{}" — should still work.
		return "pong", nil
	}
	v := NewHarnessValidator(loader, invoke)
	v.Scorer = validatorScorer()
	run, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if run.Passed != 1 {
		t.Errorf("Passed = %d, want 1", run.Passed)
	}
}

func TestHarnessValidator_RejectsEmptyDataset(t *testing.T) {
	v := NewHarnessValidator(newCaseLoader(nil), func(ctx context.Context, req LLMInvokeRequest) (string, error) { return "x", nil })
	_, err := v.Validate(context.Background(), "c1", []byte("p"), "missing-ds")
	if err == nil {
		t.Fatalf("expected error on empty dataset")
	}
}

func TestHarnessValidator_UnknownScorer(t *testing.T) {
	v := NewHarnessValidator(newCaseLoader(validatorCases()), func(ctx context.Context, req LLMInvokeRequest) (string, error) { return "x", nil })
	v.Scorer = "not_a_real_scorer"
	_, err := v.Validate(context.Background(), "c1", []byte("p"), "ds1")
	if err == nil {
		t.Fatalf("expected error on unknown scorer")
	}
}

func TestHarnessValidator_PassesSystemPromptThrough(t *testing.T) {
	loader := newCaseLoader(validatorCases())
	var sawPrompt string
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		sawPrompt = req.System
		return "pong", nil
	}
	v := NewHarnessValidator(loader, invoke)
	v.Scorer = validatorScorer()
	if _, err := v.Validate(context.Background(), "c1", []byte("THE PROMPT"), "ds1"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sawPrompt != "THE PROMPT" {
		t.Errorf("System = %q, want THE PROMPT", sawPrompt)
	}
}

// caseLoader is a tiny CaseLoader for tests.
type caseLoader struct{ cases []harness.DatasetCase }

func newCaseLoader(c []harness.DatasetCase) CaseLoader { return &caseLoader{cases: c}

func (l *caseLoader) ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error) {
	return l.cases, nil
}