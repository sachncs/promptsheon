package selfevolve

import (
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/eval"
)

// Validator scores a candidate prompt against the
// capability's dataset. Run BEFORE promoter.Promote so a
// bad candidate never reaches an Activate path. The
// default implementation calls the LLM directly for each
// case (no DB writes), scores with the chosen Scorer, and
// returns a synthetic EvalRun.
type Validator interface {
	Validate(ctx context.Context, capabilityID string, promptBytes []byte, datasetID string) (*harness.EvalRun, error)
}

// CaseLoader reads dataset cases. The validator takes
// this as a method-set so the evolver doesn't have to
// expose the full Repository.
type CaseLoader interface {
	ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error)
}

// HarnessValidator is the default Validator. It calls
// the LLM directly per case, scores the output, and
// returns a synthetic EvalRun. The run's ID and
// ReleaseID are both "self-evolve-validate-<cap>" so
// audit log entries are identifiable; the run is NOT
// persisted to the eval_runs table.
type HarnessValidator struct {
	Cases  CaseLoader
	Invoke LLMInvokeFn
	Scorer eval.Scorer
	Now    func() time.Time
}

// NewHarnessValidator constructs a validator. Scorer
// defaults to "contains" so a substring match is enough —
// the model doesn't have to produce exact wording.
func NewHarnessValidator(cases CaseLoader, invoke LLMInvokeFn) *HarnessValidator {
	return &HarnessValidator{
		Cases:  cases,
		Invoke: invoke,
		Scorer: eval.ScorerContains,
		Now:    func() time.Time { return time.Now().UTC() },
	}
}

// Validate runs the candidate prompt against the dataset
// and returns a synthetic EvalRun. The candidate is not
// activated and no row is written to eval_runs.
func (v *HarnessValidator) Validate(ctx context.Context, capabilityID string, promptBytes []byte, datasetID string) (*harness.EvalRun, error) {
	if v.Cases == nil {
		return nil, fmt.Errorf("selfevolve.validator: no case loader wired")
	}
	if v.Invoke == nil {
		return nil, fmt.Errorf("selfevolve.validator: no LLM invoke wired")
	}
	if capabilityID == "" || datasetID == "" {
		return nil, fmt.Errorf("selfevolve.validator: missing capabilityID or datasetID")
	}
	cases, err := v.Cases.ListDatasetCases(ctx, datasetID)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.validator: list cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("selfevolve.validator: dataset %s has no cases", datasetID)
	}
	scorer, ok := eval.Lookup(v.Scorer)
	if !ok {
		return nil, fmt.Errorf("selfevolve.validator: unknown scorer %q", v.Scorer)
	}
	started := v.Now()
	prompt := string(promptBytes)
	var passed, failed int
	for _, c := range cases {
		inputsJSON := string(c.Inputs)
		if inputsJSON == "" {
			inputsJSON = "{}"
		}
		out, err := v.Invoke(ctx, LLMInvokeRequest{
			System: prompt,
			User:   inputsJSON,
		})
		if err != nil {
			failed++
			continue
		}
		// Wrap the output in {"content": "..."} so the
		// scorer's contains/exact_match comparisons
		// work.
		raw, mErr := json.Marshal(map[string]any{"content": out})
		if mErr != nil {
			failed++
			continue
		}
		if ok, sErr := scorer.ScoreCase(raw, c.Expected); sErr != nil || !ok {
			failed++
			continue
		}
		passed++
	}
	total := passed + failed
	var score float64
	if total > 0 {
		score = float64(passed) / float64(total)
	}
	status := harness.RunPassed
	if failed > 0 {
		status = harness.RunFailed
	}
	finished := v.Now()
	return &harness.EvalRun{
		ID:         "self-evolve-validate-" + capabilityID + "-" + strings.ReplaceAll(started.Format(time.RFC3339Nano), ":", "-"),
		ReleaseID:  "self-evolve-validate-" + capabilityID,
		DatasetID:  datasetID,
		Scorer:     scorer.Name(),
		Score:      score,
		Passed:     passed,
		Failed:     failed,
		Total:      total,
		Status:     status,
		StartedAt:  started,
		FinishedAt: &finished,
	}, nil
}
