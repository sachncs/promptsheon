package rules

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

func baseObs() Observation {
	return Observation{
		CapabilityID:      "cap-1",
		CapabilityVersion: "ver-1",
		Environment:       "prod",
		WindowExecutions:  64,
		P95LatencyMS:      200,
		AvgCostUSDMicro:   50_000,
		HallucinationRate: 0.01,
		SuccessRate:       0.99,
	}
}

func TestNewEngineHasRules(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	if len(e.rules) == 0 {
		t.Fatalf("expected at least one rule")
	}
}

func TestNoRuleFiresOnQuietObservation(t *testing.T) {
	t.Parallel()
	got := NewEngine().Evaluate(context.Background(), baseObs())
	if len(got) != 0 {
		t.Fatalf("expected no recommendations on quiet obs, got %d", len(got))
	}
}

func TestCompressWhenSlow(t *testing.T) {
	t.Parallel()
	obs := baseObs()
	obs.P95LatencyMS = 1500
	got := NewEngine().Evaluate(context.Background(), obs)
	if !hasType(got, capability.RecommendationCompressPrompt) {
		t.Fatalf("expected compress-prompt, got %v", got)
	}
}

func TestCacheWhenCostly(t *testing.T) {
	t.Parallel()
	obs := baseObs()
	obs.AvgCostUSDMicro = 200_000
	got := NewEngine().Evaluate(context.Background(), obs)
	if !hasType(got, capability.RecommendationEnableCache) {
		t.Fatalf("expected enable-cache, got %v", got)
	}
}

func TestGuardrailWhenUngrounded(t *testing.T) {
	t.Parallel()
	obs := baseObs()
	obs.HallucinationRate = 0.08
	got := NewEngine().Evaluate(context.Background(), obs)
	if !hasType(got, capability.RecommendationAddGuardrail) {
		t.Fatalf("expected add-guardrail, got %v", got)
	}
}

func TestRulesRespectMinimumWindowSize(t *testing.T) {
	t.Parallel()
	obs := baseObs()
	obs.WindowExecutions = 4 // below the 32 threshold
	obs.P95LatencyMS = 5000
	obs.AvgCostUSDMicro = 1_000_000
	obs.HallucinationRate = 0.99
	got := NewEngine().Evaluate(context.Background(), obs)
	if len(got) != 0 {
		t.Fatalf("expected no recommendations on a small window, got %d", len(got))
	}
}

func TestCanAutoAdoptClassifiesTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		t    capability.RecommendationType
		want bool
	}{
		{capability.RecommendationCompressPrompt, true},
		{capability.RecommendationEnableCache, true},
		{capability.RecommendationAddGuardrail, false},
		{capability.RecommendationSwitchModel, false},
	}
	for _, c := range cases {
		c := c
		t.Run(string(c.t), func(t *testing.T) {
			t.Parallel()
			r := capability.Recommendation{Type: c.t, Confidence: 0.9, AutoApplicable: true}
			if got := CanAutoAdopt(r); got != c.want {
				t.Errorf("expected %v, got %v", c.want, got)
			}
		})
	}
}

func hasType(recs []capability.Recommendation, t capability.RecommendationType) bool {
	for _, r := range recs {
		if r.Type == t {
			return true
		}
	}
	return false
}
