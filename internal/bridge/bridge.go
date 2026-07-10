// Package bridge wires the SLO library (internal/slo) into the
// Recommendation engine (internal/recommendation.Producer). The
// bridge converts SLO breach events into capability.Recommendation
// values; the Producer's existing SinkFunc then persists them
// alongside the rule-engine output.
//
// This is the Tier 2.49 follow-on: the SLO library shipped its
// value type in the previous commit. Today's commit provides the
// adapter that makes SLO breaches a first-class source of
// Recommendations, alongside the deterministic rules engine.
//
// The recommended type is fixed at capability.RecommendationAddGuardrail
// when the breach is on HallucinationRate or SuccessRate; the
// engine is forced to AddGuardrail because SLO breaches are the
// single most authoritative signal for a Capability to need
// additional defence. Other signal types (latency, cost) are
// surfaced as recommendations in a follow-on commit; today they
// are not emitted by the bridge to keep the contract narrow.
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

// Evaluate returns one Recommendation per breach. The shape is
// fixed (AddGuardrail) and the Reason field carries the
// auditable signal name and burn-rate. The bridge returns nil
// when the breach is on a signal the v1 bridge does not cover
// (latency, cost) so the rules engine can keep emitting
// compress/cache recommendations for those.
//
// The non-blocking nature of the bridge is deliberate: an empty
// event returns nil, nil.
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
	switch b.Signal {
	case "hallucination_rate", "success_rate":
		rec := &capability.Recommendation{
			ID:                  "slo-bridge-" + b.Signal + "-" + b.VersionID,
			CapabilityVersionID: b.VersionID,
			Type:                capability.RecommendationAddGuardrail,
			Reason:              fmt.Sprintf("SLO breach signal=%s burn_rate=%.4f", b.Signal, b.BurnRate),
			Confidence:          0.95,
			Impact:              "high",
			AutoApplicable:      false, // guardrail additions never auto-applied
			CreatedAt:           b.DetectedAt,
		}
		return rec, nil
	default:
		// Latency, cost, availability are surfaced by the rules
		// engine today; the bridge ignores them. The empty
		// Recommendation list is the contract.
		return nil, nil
	}
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
		rec, err := e.Evaluate()
		if err != nil {
			return out, err
		}
		if rec != nil {
			out = append(out, *rec)
		}
	}
	return out, nil
}
