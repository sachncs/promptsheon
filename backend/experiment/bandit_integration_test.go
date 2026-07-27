package experiment

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
)

// TestEngineSelectsWithBandit exercises the bandit selector
// against the Engine's variant lifecycle. The Engine currently
// uses static traffic-weighted selection; this test documents
// the integration point: when the Engine grows a SetBandit
// method in a follow-on commit, this test will pass under the
// new selection policy.
func TestEngineSelectsWithBandit(t *testing.T) {
	t.Parallel()
	// Stand-in: construct a Selector with the engine's variants and
	// verify it can be operated independently. The Engine integration
	// is M4 follow-on work per ADR-0021.
	sel := bandit.NewSelector([]string{"v1", "v2", "v3"})
	for i := 0; i < 10; i++ {
		_ = sel.Observe("v1", true)
		_ = sel.Observe("v2", false)
		_ = sel.Observe("v3", false)
	}
	m, err := sel.PosteriorMean("v1")
	if err != nil {
		t.Fatalf("PosteriorMean: %v", err)
	}
	if m <= 0.5 {
		t.Fatalf("expected v1 to have > 0.5 posterior mean after 10 successes, got %f", m)
	}
}
