//go:build tests_migration


package harness_test

import (
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/eval"
)

// BenchmarkEvalRunStreaming1000Cases pins M-7 / PERF-DB-3: a
// 1000-case eval run streamed per-result (c2.22 / c0.20) must
// complete under 30 s on a 4-core box with bounded memory.
//
// The previous code accumulated every EvalResult in a slice before
// bulk-insert; at 1000 cases each holding an Actual json.RawMessage
// the process held ~1 MB of result structs. The fix in c0.20 / c2.22
// streams results via CreateEvalResult so memory stays bounded.
//
// Run with: go test -bench=BenchmarkEvalRunStreaming1000Cases
// -benchtime=1x -timeout=120s ./backend/tests/unit/harness/
func BenchmarkEvalRunStreaming1000Cases(b *testing.B) {
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
		ID:           "ds-perf-stream",
		CapabilityID: "c1",
		Name:         "perf-stream",
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
