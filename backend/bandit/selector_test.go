package bandit

import (
	"math/rand/v2"
	"testing"
)

func TestArmPosteriorMean(t *testing.T) {
	t.Parallel()
	p := NewArmPosterior()
	if got := p.Mean(); got != 0.5 {
		t.Fatalf("uniform-prior mean should be 0.5, got %f", got)
	}
	p.Observe(true)
	p.Observe(true)
	p.Observe(false)
	// 1 + 2 successes / 3 + 2 failures = 3/5
	if got := p.Mean(); got != 0.6 {
		t.Fatalf("expected 0.6, got %f", got)
	}
}

func TestArmPosteriorSampleDeterministic(t *testing.T) {
	t.Parallel()
	p := NewArmPosterior()
	p.Observe(true)
	p.Observe(false)
	p.Observe(true)
	seed := uint64(42)
	rng := rand.New(rand.NewPCG(seed, seed))
	a := p.Sample(rng)
	b := p.Sample(rng)
	// Sanity: two consecutive draws from the same RNG differ.
	if a == b {
		t.Fatalf("two consecutive draws should differ, both = %f", a)
	}
	if a < 0 || a > 1 {
		t.Fatalf("sample should be in [0, 1], got %f", a)
	}
}

func TestSelectorSelectUnknownArm(t *testing.T) {
	t.Parallel()
	s := NewSelector([]string{"a", "b"})
	if err := s.Observe("zzz", true); err != ErrUnknownArm {
		t.Fatalf("expected ErrUnknownArm, got %v", err)
	}
}

func TestSelectorSelectNoArms(t *testing.T) {
	t.Parallel()
	s := NewSelector(nil)
	if _, err := s.Select(); err != ErrNoArms {
		t.Fatalf("expected ErrNoArms, got %v", err)
	}
}

func TestSelectorExploitationAndExploration(t *testing.T) {
	t.Parallel()
	// 5-arm problem. Arm "winner" sees 100 successes; the others
	// see 0. After enough Thompson samples the selector must prefer
	// "winner" overwhelmingly (high posterior mean) but the
	// wide posteriors of the losers must still produce occasional
	// selections (exploration).
	s := NewSelector([]string{"winner", "a", "b", "c", "d"})
	for i := 0; i < 100; i++ {
		_ = s.Observe("winner", true)
	}
	for i := 0; i < 5; i++ {
		_ = s.Observe("a", false)
	}
	for i := 0; i < 5; i++ {
		_ = s.Observe("b", false)
	}
	for i := 0; i < 5; i++ {
		_ = s.Observe("c", false)
	}
	for i := 0; i < 5; i++ {
		_ = s.Observe("d", false)
	}
	rng := rand.New(rand.NewPCG(1, 1))
	counts := map[string]int{}
	for i := 0; i < 200; i++ {
		arm, err := s.SelectWithRNG(rng)
		if err != nil {
			t.Fatalf("SelectWithRNG: %v", err)
		}
		counts[arm]++
	}
	if counts["winner"] < 100 {
		t.Fatalf("expected winner selected at least 100 times in 200 trials, got %d", counts["winner"])
	}
}

func TestPosteriorMeanUnknownArm(t *testing.T) {
	t.Parallel()
	s := NewSelector([]string{"a"})
	if _, err := s.PosteriorMean("zzz"); err != ErrUnknownArm {
		t.Fatalf("expected ErrUnknownArm, got %v", err)
	}
}
