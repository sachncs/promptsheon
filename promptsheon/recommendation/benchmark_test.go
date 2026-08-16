// Package recommendation — benchmark for the recommendation
// selection path. PR-2 of the audit-fixes plan: the curated
// bench list (scripts/benchmarks.txt) requires BenchmarkSelect,
// which was originally written for the bandit/Thompson-sampling
// selection path. The bandit code was retired in the
// compliance refactor in favour of the deterministic rules
// engine in promptsheon/rules; this benchmark covers the
// selection path that production actually exercises today.
//
// Path covered:
//   Producer.Tick -> aggregator -> rules.Engine.Evaluate
// The benchmark calls Evaluate directly because Tick also
// touches the observation aggregator and eventbus publisher,
// both of which have their own benchmarks (BenchmarkAppendAuditCAS*
// for the audit pipeline; the eventbus publisher has no bench
// today because it's a thin in-memory fan-out — see todo.md
// G-series for the deferred compliance items). Benchmarking the
// selection in isolation is the right scope for a curated gate:
// every recommendation decision runs through Evaluate, and the
// time spent there is the upper bound on selection latency for
// the loop.
package recommendation_test

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/rules"
)

// BenchmarkSelect exercises the recommendation selection path.
// A representative observation is constructed once per
// invocation; Evaluate runs every registered rule and merges
// the results. The benchmark pins the "recommendation loop"
// hot path: each Tick produces a recommendation from the
// observation window; the rule engine decides what to emit.
//
// b.ReportAllocs and a sink variable keep the compiler from
// optimising the result away (Go's escape analysis would
// otherwise eliminate the allocations that dominate runtime
// cost in production).
func BenchmarkSelect(b *testing.B) {
	ctx := context.Background()
	engine := rules.NewEngine()
	obs := rules.Observation{
		CapabilityID:      "bench-cap",
		CapabilityVersion: "v1.0.0",
		Environment:       "bench-env",
		WindowExecutions:  1000,
		P95LatencyMS:      420,
		AvgCostUSDMicro:   250_000, // $0.25 / execution
		HallucinationRate: 0.04,
		SuccessRate:       0.97,
	}

	var sink int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recs := engine.Evaluate(ctx, obs)
		// Touch the result so the compiler cannot prove the
		// allocation is dead.
		sink += len(recs)
	}
	// Prevent sink from being optimised out entirely.
	if sink < 0 {
		b.Fatal("sink overflow")
	}
}
