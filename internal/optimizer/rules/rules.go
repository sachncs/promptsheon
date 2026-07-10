// Package rules implements the deterministic recommendation engine
// for Promptsheon. M4 ships the v1 engine: a small, fast, auditable
// set of "if X then suggest Y" rules that look at recent Execution
// windows and emit Recommendation values when thresholds are
// breached. The engine is deliberately not LLM-driven or
// bandit-driven in v1; the goal is to close the recommendation
// loop end-to-end so the Decision and lineage paths can be wired,
// then substitute in a more sophisticated engine in M4+.
//
// Each rule is a plain function; the engine wires them together.
// New rules can be added by appending to the rules slice in
// NewEngine — no other code changes needed.
package rules

import (
	"context"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/recommendation"
)

// Observation is the minimal signal a rule consumes. The full
// observation aggregate (Execution-windowed stats) is built in a
// later milestone; v1 rules only need a few scalars.
type Observation struct {
	CapabilityID      string
	CapabilityVersion string
	Environment       string
	WindowExecutions  int64
	P95LatencyMS      int64
	AvgCostUSDMicro   int64 // cost in micro-USD to avoid float arithmetic
	HallucinationRate float64
	SuccessRate       float64
}

// Engine executes a fixed list of rules against an Observation
// and returns the Recommendations they emit.
type Engine struct {
	rules []Rule
}

// Rule is one deterministic suggestion. The engine passes the
// observation in; the rule decides whether to emit any
// Recommendations for it.
type Rule func(ctx context.Context, obs Observation) []capability.Recommendation

// NewEngine returns an engine pre-loaded with the v1 rule set:
//   - compress prompt when p95 latency is high
//   - enable cache when cost per execution is high
//   - switch guardrail when hallucination is high
//
// Adding rules later is a one-line append to the slice.
func NewEngine() *Engine {
	return &Engine{rules: []Rule{
		compressWhenSlow,
		cacheWhenCostly,
		guardrailWhenUngrounded,
	}}
}

// Evaluate runs every rule and merges their outputs. The merge
// step is a simple append because rules emit distinct
// RecommendationType values; deduplication is left to the
// Recommendation store (M0.6) which dedupes by CapabilityVersionID
// + Type.
func (e *Engine) Evaluate(ctx context.Context, obs Observation) []capability.Recommendation {
	var out []capability.Recommendation
	for _, r := range e.rules {
		out = append(out, r(ctx, obs)...)
	}
	return out
}

// CanAutoAdopt gates the engine output against the conservative
// defaults shipped in recommendation.CanAutoAdopt. Centralised
// here so callers do not have to import recommendation.
func CanAutoAdopt(r capability.Recommendation) bool {
	return recommendation.CanAutoAdopt(r, 0.8)
}

// compressWhenSlow emits a compress-prompt Recommendation when the
// 95th-percentile latency breaches one second.
func compressWhenSlow(_ context.Context, obs Observation) []capability.Recommendation {
	if obs.P95LatencyMS < 1000 || obs.WindowExecutions < 32 {
		return nil
	}
	return []capability.Recommendation{{
		Type:                 capability.RecommendationCompressPrompt,
		CapabilityVersionID:  obs.CapabilityVersion,
		Reason:               "P95 latency > 1s across recent executions",
		Confidence:           0.7,
		Impact:               "medium",
		AutoApplicable:       true,
	}}
}

// cacheWhenCostly emits an enable-cache Recommendation when the
// average cost per execution exceeds 100 micro-USD (i.e. one cent
// per 100 executions; small but worth fixing).
func cacheWhenCostly(_ context.Context, obs Observation) []capability.Recommendation {
	if obs.AvgCostUSDMicro < 100_000 || obs.WindowExecutions < 32 {
		return nil
	}
	return []capability.Recommendation{{
		Type:                 capability.RecommendationEnableCache,
		CapabilityVersionID:  obs.CapabilityVersion,
		Reason:               "Average cost > 1c per 100 executions",
		Confidence:           0.85,
		Impact:               "medium",
		AutoApplicable:       true,
	}}
}

// guardrailWhenUngrounded emits an add-guardrail Recommendation when
// the hallucination rate exceeds 5% on a window of >=32 executions.
func guardrailWhenUngrounded(_ context.Context, obs Observation) []capability.Recommendation {
	if obs.WindowExecutions < 32 || obs.HallucinationRate < 0.05 {
		return nil
	}
	return []capability.Recommendation{{
		Type:                 capability.RecommendationAddGuardrail,
		CapabilityVersionID:  obs.CapabilityVersion,
		Reason:               "Hallucination rate above 5%",
		Confidence:           0.8,
		Impact:               "high",
		AutoApplicable:       false, // adds a guardrail — never auto
	}}
}
