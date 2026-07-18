package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestBreachEventEmpty(t *testing.T) {
	t.Parallel()
	if _, err := (BreachEvent{}).Evaluate(); err == nil {
		t.Fatalf("expected error for empty capability_id")
	}
}

func TestBreachEventZeroBurnRate(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", BurnRate: 0}
	got, err := b.Evaluate()
	if err != nil {
		t.Fatalf("zero burn rate should not error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil recommendation for zero burn rate")
	}
}

func TestBreachEventHallucinationProducesAddGuardrail(t *testing.T) {
	t.Parallel()
	b := BreachEvent{
		CapabilityID: "cap-1",
		VersionID:    "ver-1",
		Signal:       "hallucination_rate",
		BurnRate:     1.5,
		DetectedAt:   time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}
	rec, err := b.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec == nil {
		t.Fatalf("expected non-nil recommendation for hallucination breach")
	}
	if rec.Type != capability.RecommendationAddGuardrail {
		t.Fatalf("expected AddGuardrail, got %s", rec.Type)
	}
	if rec.AutoApplicable {
		t.Fatalf("guardrail additions must not be auto-applicable")
	}
	if rec.Confidence < 0.9 {
		t.Fatalf("expected high confidence, got %f", rec.Confidence)
	}
}

func TestBreachEventSuccessRateProducesAddGuardrail(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "success_rate", BurnRate: 1.2}
	rec, err := b.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec == nil || rec.Type != capability.RecommendationAddGuardrail {
		t.Fatalf("expected AddGuardrail for success_rate, got %+v", rec)
	}
}

func TestBreachEventLatencyMapsToCompressPrompt(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "p95_latency_ms", BurnRate: 1.5}
	rec, err := b.recommendation()
	if err != nil {
		t.Fatalf("recommendation: %v", err)
	}
	if rec == nil {
		t.Fatal("expected recommendation for latency breach")
	}
	if rec.Type != "compress_prompt" {
		t.Errorf("Type = %q, want compress_prompt", rec.Type)
	}
}

func TestBreachEventHighLatencyEscalatesToReduceContext(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "p95_latency_ms", BurnRate: 5.0}
	rec, err := b.recommendation()
	if err != nil {
		t.Fatalf("recommendation: %v", err)
	}
	if rec == nil {
		t.Fatal("expected recommendation for latency breach")
	}
	if rec.Type != "reduce_context" {
		t.Errorf("Type = %q, want reduce_context", rec.Type)
	}
}

func TestBreachEventCostMapsToEnableCache(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "avg_cost_usd_micro", BurnRate: 1.2}
	rec, err := b.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec == nil {
		t.Fatal("expected recommendation for cost breach")
	}
	if rec.Type != "enable_cache" {
		t.Errorf("Type = %q, want enable_cache", rec.Type)
	}
	if !rec.AutoApplicable {
		t.Error("enable_cache should be AutoApplicable=true")
	}
}

func TestBreachEventAvailabilityMapsToTunePolicy(t *testing.T) {
	t.Parallel()
	b := BreachEvent{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "availability", BurnRate: 3.0}
	rec, err := b.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec == nil {
		t.Fatal("expected recommendation for availability breach")
	}
	if rec.Type != "tune_policy" {
		t.Errorf("Type = %q, want tune_policy", rec.Type)
	}
}

func TestRunAggregatesEvents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	events := []BreachEvent{
		{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "hallucination_rate", BurnRate: 1.5, DetectedAt: now},
		{CapabilityID: "cap-1", VersionID: "ver-1", Signal: "p95_latency_ms", BurnRate: 2.0, DetectedAt: now},
		{CapabilityID: "cap-2", VersionID: "ver-2", Signal: "success_rate", BurnRate: 1.2, DetectedAt: now},
	}
	got, err := Run(context.Background(), events)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 recommendations (hallucination + latency + success_rate), got %d", len(got))
	}
}

func TestRunRespectsContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, []BreachEvent{{CapabilityID: "cap", VersionID: "ver", Signal: "hallucination_rate", BurnRate: 1.0}})
	if err == nil {
		t.Fatalf("expected context canceled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
