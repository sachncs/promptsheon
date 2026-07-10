package recommendation

import (
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestNewAdoptedRequiresVersion(t *testing.T) {
	t.Parallel()
	if _, err := NewAdopted("rec-1", "alice", "looks good", 0); err == nil {
		t.Fatalf("expected error for resulting_version=0")
	}
}

func TestNewAdoptedRequiresIdentity(t *testing.T) {
	t.Parallel()
	if _, err := NewAdopted("rec-1", "", "looks good", 7); err == nil {
		t.Fatalf("expected error for empty decided_by")
	}
}

func TestNewRejected(t *testing.T) {
	t.Parallel()
	d, err := NewRejected("rec-1", "alice", "defer")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if d.Outcome != OutcomeRejected {
		t.Fatalf("expected Rejected, got %s", d.Outcome)
	}
	if d.Auto {
		t.Fatalf("expected Auto=false")
	}
}

func TestNewAdoptedAuto(t *testing.T) {
	t.Parallel()
	d, err := NewAdoptedAuto("rec-1", 7)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if !d.Auto {
		t.Fatalf("expected Auto=true")
	}
	if d.DecidedBy != "system:auto-optimizer" {
		t.Fatalf("expected system identity, got %s", d.DecidedBy)
	}
}

func TestNewSuperseded(t *testing.T) {
	t.Parallel()
	d, err := NewSuperseded("rec-1")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if d.Outcome != OutcomeSuperseded {
		t.Fatalf("expected Superseded, got %s", d.Outcome)
	}
	if d.ResultingVersion != 0 {
		t.Fatalf("expected zero version, got %d", d.ResultingVersion)
	}
}

func TestNewRejectsUnknownOutcome(t *testing.T) {
	t.Parallel()
	if _, err := newDecision("rec-1", Outcome("maybe"), "alice", "", 0, false); !errors.Is(err, ErrUnknownOutcome) {
		t.Fatalf("expected ErrUnknownOutcome, got %v", err)
	}
}

func TestNewRejectsEmptyRecommendationID(t *testing.T) {
	t.Parallel()
	if _, err := NewRejected("", "alice", ""); err == nil {
		t.Fatalf("expected error for empty recommendation_id")
	}
}

func TestCanAutoAdoptMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		rec   capability.Recommendation
		allow bool
	}{
		{
			name:  "compress at high confidence allowed",
			rec:   capability.Recommendation{Type: capability.RecommendationCompressPrompt, AutoApplicable: true, Confidence: 0.9},
			allow: true,
		},
		{
			name:  "switch_model never auto",
			rec:   capability.Recommendation{Type: capability.RecommendationSwitchModel, AutoApplicable: true, Confidence: 0.99},
			allow: false,
		},
		{
			name:  "auto_applicable false rejected",
			rec:   capability.Recommendation{Type: capability.RecommendationCompressPrompt, AutoApplicable: false, Confidence: 0.99},
			allow: false,
		},
		{
			name:  "below threshold rejected",
			rec:   capability.Recommendation{Type: capability.RecommendationCompressPrompt, AutoApplicable: true, Confidence: 0.5},
			allow: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CanAutoAdopt(tc.rec, 0.8)
			if got != tc.allow {
				t.Fatalf("expected %v, got %v", tc.allow, got)
			}
		})
	}
}
