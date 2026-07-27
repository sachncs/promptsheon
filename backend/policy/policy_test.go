package policy

import (
	"testing"
	"time"
)

func TestAllowedProviders(t *testing.T) {
	t.Parallel()
	pol := AllowedProviders{
		PolicyName: "providers",
		Providers:  map[string]struct{}{"openai": {}},
	}
	if got := pol.Evaluate(ManifestPolicyInput{Provider: "openai"}); got.Effect != EffectAllow {
		t.Fatalf("openai should be allowed, got %s", got.Effect)
	}
	if got := pol.Evaluate(ManifestPolicyInput{Provider: "bogus"}); got.Effect != EffectDeny {
		t.Fatalf("bogus should be denied, got %s", got.Effect)
	}
}

func TestCostCeiling(t *testing.T) {
	t.Parallel()
	pol := CostCeiling{PolicyName: "cost", MaxUSD: 1.0}
	if got := pol.Evaluate(ExecutionPolicyInput{EstimatedUSD: 0.5}); got.Effect != EffectAllow {
		t.Fatalf("under ceiling should allow, got %s", got.Effect)
	}
	if got := pol.Evaluate(ExecutionPolicyInput{EstimatedUSD: 2.0}); got.Effect != EffectDeny {
		t.Fatalf("over ceiling should deny, got %s", got.Effect)
	}
	if got := pol.Evaluate(ReleasePolicyInput{}); got.Effect != EffectDeny {
		t.Fatalf("non-execution input must deny (fail-closed)")
	}
}

func TestPIIRedaction(t *testing.T) {
	t.Parallel()
	pol := PIIRedaction{PolicyName: "pii"}
	if got := pol.Evaluate(ManifestPolicyInput{HasPII: true}); got.Effect != EffectRedact {
		t.Fatalf("PII manifests should redact, got %s", got.Effect)
	}
	if got := pol.Evaluate(ManifestPolicyInput{HasPII: false}); got.Effect != EffectAllow {
		t.Fatalf("clean manifests should allow, got %s", got.Effect)
	}
}

func TestDataResidency(t *testing.T) {
	t.Parallel()
	pol := DataResidency{PolicyName: "res", Region: "eu-west-1"}
	if got := pol.Evaluate(ManifestPolicyInput{DataRegion: "eu-west-1"}); got.Effect != EffectAllow {
		t.Fatalf("matching region should allow, got %s", got.Effect)
	}
	if got := pol.Evaluate(ManifestPolicyInput{DataRegion: "us-east-1"}); got.Effect != EffectDeny {
		t.Fatalf("non-matching region should deny, got %s", got.Effect)
	}
}

func TestChangeWindow(t *testing.T) {
	t.Parallel()
	pol, err := NewChangeWindow("window", 9, 17, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	friday := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if got := pol.Evaluate(ReleasePolicyInput{Time: friday}); got.Effect != EffectAllow {
		t.Fatalf("weekday noon should allow, got %s", got.Effect)
	}
	sunday := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	if got := pol.Evaluate(ReleasePolicyInput{Time: sunday}); got.Effect != EffectDeny {
		t.Fatalf("Sunday should deny, got %s", got.Effect)
	}
	friday_evening := time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC)
	if got := pol.Evaluate(ReleasePolicyInput{Time: friday_evening}); got.Effect != EffectDeny {
		t.Fatalf("after-hours Friday should deny, got %s", got.Effect)
	}
	if got := pol.Evaluate(ReleasePolicyInput{Time: friday, BypassWindow: true}); got.Effect != EffectAllow {
		t.Fatalf("bypass flag should allow, got %s", got.Effect)
	}
}

func TestChangeWindowRejectsBadInputs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func() (ChangeWindow, error)
	}{
		{"negative hour", func() (ChangeWindow, error) { return NewChangeWindow("w", -1, 17, time.Monday) }},
		{"toHour out of range", func() (ChangeWindow, error) { return NewChangeWindow("w", 9, 25, time.Monday) }},
		{"from >= to", func() (ChangeWindow, error) { return NewChangeWindow("w", 17, 9, time.Monday) }},
		{"no days", func() (ChangeWindow, error) { return NewChangeWindow("w", 9, 17) }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tc.fn(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestBundleShortCircuits(t *testing.T) {
	t.Parallel()
	b := Bundle{
		Manifest: []Policy{
			AllowedProviders{PolicyName: "providers", Providers: map[string]struct{}{"openai": {}}},
			PIIRedaction{PolicyName: "pii"},
		},
	}
	if got := b.EvaluateManifest(ManifestPolicyInput{Provider: "openai", HasPII: true}); got.Effect != EffectRedact {
		t.Fatalf("expected Redact downstream, got %s", got.Effect)
	}
	if got := b.EvaluateManifest(ManifestPolicyInput{Provider: "bogus"}); got.Effect != EffectDeny {
		t.Fatalf("expected Deny upstream, got %s", got.Effect)
	}
}
