// Package slo declares Capability-level Service Level Objectives.
//
// A Capability SLO is a Goal (target) on a Signal (latency, cost,
// success rate, hallucination rate) over a Window (rolling
// duration). The SLO library ships the value type, evaluation,
// and a burn-rate alert helper used by the policy bundle and
// Recommendation engine v2.
//
// This is Tier 2.49 of the architecture review board; the
// Recommendation engine v2 (bandit) is in M4 follow-on work
// (ADR-0019).
package slo

import (
	"context"
	"fmt"
	"time"
)

// interfaceCtx aliases context.Context to avoid a file-scoped import
// cycle when the Repository methods are reproduced below. It is
// identical to context.Context.
type interfaceCtx = context.Context

// Signal enumerates the metrics the SLO can target.
type Signal string

const (
	SignalP95LatencyMS      Signal = "p95_latency_ms"
	SignalP99LatencyMS      Signal = "p99_latency_ms"
	SignalSuccessRate       Signal = "success_rate"
	SignalHallucinationRate Signal = "hallucination_rate"
	SignalAvgCostMicroUSD   Signal = "avg_cost_usd_micro"
	SignalAvailability      Signal = "availability"
)

// Window is the rolling duration the SLO is measured over.
type Window time.Duration

const (
	Window5Min  Window = Window(5 * time.Minute)
	Window1Hour Window = Window(time.Hour)
	Window1Day  Window = Window(24 * time.Hour)
)

// Goal declares a target on a Signal. "value <" or "value >"
// combined with Target produces the comparison the SLO evaluator
// applies.
type Goal struct {
	Signal Signal
	Op     Op
	Target float64
	Window Window
	Breach float64 // burn-rate threshold: 1.0 means any breach alerts
}

// Op is the comparison operator on a target value.
type Op string

const (
	OpLT  Op = "<"
	OpLTE Op = "<="
	OpGT  Op = ">"
	OpGTE Op = ">="
)

// SLO is one Service Level Objective. Each Capability carries zero
// or more SLOs; an SLO breach fires an alert and (optionally) a
// Recommendation.
type SLO struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	CapabilityID string    `json:"capability_id"`
	Goal         Goal      `json:"goal"`
	Severity     string    `json:"severity"` // page | ticket | log
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
}

// Errorf-from-validation path: every SLO must specify a Signal,
// Op, Target, Window, and a positive Breach.
var (
	ErrInvalidSignal = fmt.Errorf("slo: invalid signal")
	ErrInvalidTarget = fmt.Errorf("slo: invalid target")
)

// Validate checks the SLO has the fields the evaluator needs.
func (s SLO) Validate() error {
	switch s.Goal.Signal {
	case SignalP95LatencyMS, SignalP99LatencyMS,
		SignalSuccessRate, SignalHallucinationRate,
		SignalAvgCostMicroUSD, SignalAvailability:
	default:
		return ErrInvalidSignal
	}
	switch s.Goal.Op {
	case OpLT, OpLTE, OpGT, OpGTE:
	default:
		return fmt.Errorf("slo: invalid op %q", s.Goal.Op)
	}
	switch s.Goal.Window {
	case Window5Min, Window1Hour, Window1Day:
	default:
		return fmt.Errorf("slo: invalid window %s", time.Duration(s.Goal.Window))
	}
	if s.Goal.Target <= 0 {
		return ErrInvalidTarget
	}
	if s.Goal.Breach <= 0 {
		return fmt.Errorf("slo: breach must be > 0, got %f", s.Goal.Breach)
	}
	return nil
}

// Evaluate checks an actual value against the SLO goal. Returns
// nil if the SLO is satisfied; returns an Err if the breach ratio
// (actual / target) exceeds the configured breach threshold.
//
// For OpLT/OpLTE the rule is "actual < target satisfied": a 500ms
// p95 latency SLO with target=1000 would be satisfied at
// actual=200 and breached at actual=1500.
//
// For OpGT/OpGTE the rule is "actual > target satisfied": a 99.9%
// availability SLO with target=0.999 would be satisfied at
// actual=0.9995 and breached at actual=0.995.
func (s SLO) Evaluate(actual float64) error {
	if err := s.Validate(); err != nil {
		return err
	}
	switch s.Goal.Op {
	case OpLT:
		if actual >= s.Goal.Target {
			return fmt.Errorf("slo %s breached: %.4f >= target %.4f", s.ID, actual, s.Goal.Target)
		}
	case OpLTE:
		if actual > s.Goal.Target {
			return fmt.Errorf("slo %s breached: %.4f > target %.4f", s.ID, actual, s.Goal.Target)
		}
	case OpGT:
		if actual <= s.Goal.Target {
			return fmt.Errorf("slo %s breached: %.4f <= target %.4f", s.ID, actual, s.Goal.Target)
		}
	case OpGTE:
		if actual < s.Goal.Target {
			return fmt.Errorf("slo %s breached: %.4f < target %.4f", s.ID, actual, s.Goal.Target)
		}
	}
	return nil
}

// BurnRate computes the ratio actual / target for an OpLT goal.
// For OpGT goals, the inverse is appropriate; callers should
// pass actual and target through Evaluate first, then decide
// whether to alert on BurnRate > Breach.
func (g Goal) BurnRate(actual float64) float64 {
	if g.Target == 0 {
		return 0
	}
	return actual / g.Target
}

// Repository is the consumer-defined persistence interface for SLOs.
type Repository interface {
	CreateSLO(ctx context.Context, s *SLO) error
	GetSLO(ctx context.Context, id string) (*SLO, error)
	ListSLOsForCapability(ctx context.Context, capabilityID string) ([]*SLO, error)
	UpdateSLO(ctx context.Context, s *SLO) error
	DeleteSLO(ctx context.Context, id string) error
}
