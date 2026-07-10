package slo

import (
	"testing"
	"time"
)

func sampleSLO(op Op, target float64) SLO {
	return SLO{
		ID:           "slo-1",
		WorkspaceID:  "ws-1",
		CapabilityID: "cap-1",
		Goal: Goal{
			Signal: SignalP95LatencyMS,
			Op:     op,
			Target: target,
			Window: Window1Hour,
			Breach: 1.0,
		},
		Severity:  "ticket",
		CreatedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		CreatedBy: "alice",
	}
}

func TestValidateRejectsUnknownSignal(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Signal = "not_a_signal"
	if err := s.Validate(); err != ErrInvalidSignal {
		t.Fatalf("expected ErrInvalidSignal, got %v", err)
	}
}

func TestValidateRejectsUnknownOp(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Op = "?="
	if err := s.Validate(); err == nil {
		t.Fatalf("expected error for unknown op")
	}
}

func TestValidateRejectsNonPositiveTarget(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Target = 0
	if err := s.Validate(); err != ErrInvalidTarget {
		t.Fatalf("expected ErrInvalidTarget, got %v", err)
	}
}

func TestEvaluateLTSatisfied(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	if err := s.Evaluate(500); err != nil {
		t.Fatalf("500ms < 1000ms target should satisfy, got %v", err)
	}
}

func TestEvaluateLTBreach(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	if err := s.Evaluate(1500); err == nil {
		t.Fatalf("1500ms >= 1000ms target should breach, got nil")
	}
}

func TestEvaluateLTEBoundary(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLTE, 1000)
	if err := s.Evaluate(1000); err != nil {
		t.Fatalf("1000ms LTE target should be satisfied at exactly 1000, got %v", err)
	}
	if err := s.Evaluate(1001); err == nil {
		t.Fatalf("1001ms LTE target should breach, got nil")
	}
}

func TestEvaluateGTSatisfied(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpGT, 0.999)
	if err := s.Evaluate(0.9995); err != nil {
		t.Fatalf("0.9995 > 0.999 should satisfy, got %v", err)
	}
}

func TestEvaluateGTEBreach(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpGTE, 0.999)
	if err := s.Evaluate(0.995); err == nil {
		t.Fatalf("0.995 < 0.999 target should breach, got nil")
	}
}

func TestEvaluateRejectsInvalidSLO(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Signal = "not_a_signal"
	if err := s.Evaluate(100); err == nil {
		t.Fatalf("Evaluate on invalid SLO must surface validation error, got nil")
	}
}

func TestBurnRateComputesRatio(t *testing.T) {
	t.Parallel()
	g := Goal{Signal: SignalP95LatencyMS, Op: OpLT, Target: 1000, Window: Window1Hour, Breach: 1.0}
	if got := g.BurnRate(1500); got != 1.5 {
		t.Fatalf("expected 1.5, got %f", got)
	}
	if got := g.BurnRate(0); got != 0 {
		t.Fatalf("expected 0/1000 = 0, got %f", got)
	}
}

func TestGoalValidateRejectsBadWindow(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Window = Window(42 * time.Hour)
	if err := s.Validate(); err == nil {
		t.Fatalf("expected error for bad window")
	}
}

func TestGoalValidateRejectsNonPositiveBreach(t *testing.T) {
	t.Parallel()
	s := sampleSLO(OpLT, 1000)
	s.Goal.Breach = 0
	if err := s.Validate(); err == nil {
		t.Fatalf("expected error for non-positive breach")
	}
}
