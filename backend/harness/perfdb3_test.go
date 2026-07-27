//go:build perfdb3
// +build perfdb3

package harness_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/eval"
	"github.com/sachncs/promptsheon/internal/harness"
)

// BenchmarkEvalRun1000Cases pins PERF-DB-3: a 1000-case eval run
// must complete under 30 s on a 4-core box. The stub invoker
// returns immediately so the budget is fully attributable to
// the runner + repo path, not the upstream LLM.
//
// Run with: go test -tags=perfdb3 -bench=BenchmarkEvalRun1000Cases
// -benchtime=1x -timeout=120s ./internal/harness/
func BenchmarkEvalRun1000Cases(b *testing.B) {
	const numCases = 1000
	r := harness.NewEvalRunner(nil, nil)
	r.Clock = func() time.Time { return time.Now().UTC() }
	repo := newRepo(b)
	r.Repo = repo
	cases := make([]harness.DatasetCase, numCases)
	for i := 0; i < numCases; i++ {
		cases[i] = harness.DatasetCase{
			ID:       fmt.Sprintf("case-%d", i),
			Seq:      i,
			Inputs:   json.RawMessage(`"hi"`),
			Expected: json.RawMessage(`"hi"`),
		}
	}
	d := &harness.Dataset{
		ID:           "ds-perf",
		CapabilityID: "c1",
		Name:         "perf",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateDataset(context.Background(), d); err != nil {
		b.Fatal(err)
	}
	if err := repo.UpsertDatasetCases(context.Background(), d.ID, cases); err != nil {
		b.Fatal(err)
	}
	r.Inv = &stubInvoker{fixed: json.RawMessage(`"hi"`)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.Run(context.Background(), harness.EvalRunOptions{
			ReleaseID:  "r1",
			DatasetID:  d.ID,
			ScorerName: eval.ScorerExactMatch,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
