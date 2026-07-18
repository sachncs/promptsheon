// Package bridge wires the SLO library (internal/slo) into the
// Recommendation engine (internal/recommendation.Producer). The
// bridge converts SLO breach events into capability.Recommendation
// values; the Producer's existing SinkFunc then persists them
// alongside the rule-engine output.
//
// Each breach signal maps to a RecommendationType:
//
//   - hallucination_rate, success_rate -> add_guardrail
//   - p95_latency_ms                  -> compress_prompt or
//     reduce_context
//   - avg_cost_usd_micro              -> enable_cache
//   - availability                    -> tune_policy
//
// The mapping is the single source of truth for "what does an SLO
// breach recommend". Production wiring runs Run() on the
// observation tick; the resulting recommendations feed the same
// Producer pipeline as the rule-engine output.
package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// BreachEvent is the input to the bridge. Construct it from
// observation rollups; the bridge produces zero or one
// Recommendations per event.
type BreachEvent struct {
	CapabilityID string
	VersionID    string
	Environment  string
	Signal       string
	BurnRate     float64
	DetectedAt   time.Time
}

// Evaluate returns one Recommendation per breach. The Reason
// field carries the auditable signal name and burn-rate. The
// bridge returns (nil, nil) when the breach is on a signal it
// does not cover so the caller can keep emitting complementary
// recommendations from other engines.
func (b BreachEvent) Evaluate() (*capability.Recommendation, error) {
	if b.CapabilityID == "" {
		return nil, fmt.Errorf("bridge: empty capability_id")
	}
	if b.VersionID == "" {
		return nil, fmt.Errorf("bridge: empty version_id")
	}
	if b.BurnRate <= 0 {
		return nil, nil
	}
	recType, ok := signalToRecommendation[b.Signal]
	if !ok {
		return nil, nil
	}
	rec := &capability.Recommendation{
		ID:                  "slo-bridge-" + b.Signal + "-" + b.VersionID,
		CapabilityVersionID: b.VersionID,
		Type:                recType,
		Reason:              fmt.Sprintf("SLO breach signal=%s burn_rate=%.4f", b.Signal, b.BurnRate),
		Confidence:          0.95,
		Impact:              "high",
		AutoApplicable:      recType == capability.RecommendationEnableCache, // cache is the only safe auto-apply
		CreatedAt:           b.DetectedAt,
	}
	return rec, nil
}

// signalToRecommendation maps an SLO signal name to the
// RecommendationType the bridge emits. Latency breaches at high
// p95 get compress_prompt; very-high p95 escalates to
// reduce_context. The boundary is governed by the burn rate:
// burn >= 2.0 means the prompt itself is too long, otherwise the
// history window is the target.
var signalToRecommendation = func() map[string]capability.RecommendationType {
	m := map[string]capability.RecommendationType{
		"hallucination_rate": capability.RecommendationAddGuardrail,
		"success_rate":       capability.RecommendationAddGuardrail,
		"avg_cost_usd_micro": capability.RecommendationEnableCache,
		"availability":       capability.RecommendationTunePolicy,
	}
	// Latency has two recommendation shapes; the per-event
	// evaluator picks one based on burn rate.
	m["p95_latency_ms"] = capability.RecommendationCompressPrompt
	return m
}()

// Evaluate is called as a method on BreachEvent so the latency
// signal can branch on burn rate without polluting the static map.
func (b BreachEvent) recTypeForLatency() capability.RecommendationType {
	if b.BurnRate >= 2.0 {
		return capability.RecommendationReduceContext
	}
	return capability.RecommendationCompressPrompt
}

// Evaluate returns the recommendation for this breach. The public
// entry point; the (signal -> type) map covers the simple cases,
// and latency routes through recTypeForLatency for burn-rate
// escalation.
func (b BreachEvent) recommendation() (*capability.Recommendation, error) {
	if b.Signal == "p95_latency_ms" {
		recType := b.recTypeForLatency()
		return &capability.Recommendation{
			ID:                  "slo-bridge-p95_latency_ms-" + b.VersionID,
			CapabilityVersionID: b.VersionID,
			Type:                recType,
			Reason:              fmt.Sprintf("SLO breach signal=%s burn_rate=%.4f", b.Signal, b.BurnRate),
			Confidence:          0.95,
			Impact:              "high",
			AutoApplicable:      false,
			CreatedAt:           b.DetectedAt,
		}, nil
	}
	return b.Evaluate()
}

// Run consumes a stream of breach events (typically the
// observation window) and produces one Recommendation per breach.
// The function is the canonical entry point for tests and for
// the production wiring's ticker.
func Run(ctx context.Context, events []BreachEvent) ([]capability.Recommendation, error) {
	out := make([]capability.Recommendation, 0, len(events))
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		rec, err := e.recommendation()
		if err != nil {
			return out, err
		}
		if rec != nil {
			out = append(out, *rec)
		}
	}
	return out, nil
}
