// Property-based tests for the bandit package. The library used
// (testing/quick) is the standard-library property harness; this
// file ships invariants and a deterministic-selection regression
// net:
//
//  1. Posterior counts track every observation exactly.
//  2. Posterior mean stays in [0, 1] regardless of input.
//  3. Unknown arms are rejected with ErrUnknownArm.
//  4. Every arm Select() and SelectWithRNG returns is in the
//     registered set.
//  5. SelectWithRNG is fully deterministic given a fixed seed.
//  6. Thompson Sampling recovers a winning arm when the
//     posterior is sufficiently peaked.
//  7. Observe never mutates the registered arm order.
//
// SelectWithRNG is the production-seed seam; Select() uses the
// per-selector RNG seeded at construction, so the same
// "registered arm" invariant applies to both call paths.
//
// The Generator interface in testing/quick uses math/rand (v1);
// we import v1 under the alias "rand" so the generator signature
// matches the interface, and import math/rand/v2 under "randv2"
// for the bandit code which is v2-based.
package bandit

import (
	rand "math/rand"
	randv2 "math/rand/v2"
	"reflect"
	"testing"
	"testing/quick"
)

// arbArmPair returns a printable Generate so we can property-test
// (armID, []bool) tuples. armID is a short printable string so
// failure messages are useful.
type arbArmPair struct {
	Arm     string
	Success []bool
}

func (arbArmPair) Generate(rng *rand.Rand, size int) reflect.Value {
	n := size
	if n > 64 {
		n = 64
	}
	success := make([]bool, n)
	for i := range success {
		success[i] = rng.Intn(2) == 0
	}
	// Two-character armIDs avoid collisions while keeping
	// failure messages short.
	arm := string([]byte{byte('a' + rng.Intn(26)), byte('a' + rng.Intn(26))})
	return reflect.ValueOf(arbArmPair{Arm: arm, Success: success})
}

// TestProperty_PosteriorCountsTrackObservations asserts that, for
// any sequence of s successes and f failures, alpha = s+1 and
// beta = f+1. The Beta(1, 1) prior is the only source of the "+1".
func TestProperty_PosteriorCountsTrackObservations(t *testing.T) {
	t.Parallel()
	f := func(p arbArmPair) bool {
		if p.Arm == "" {
			return true
		}
		sel := NewSelector([]string{p.Arm})
		s, fCount := 0, 0
		for _, ok := range p.Success {
			if err := sel.Observe(p.Arm, ok); err != nil {
				t.Logf("Observe: %v", err)
				return false
			}
			if ok {
				s++
			} else {
				fCount++
			}
		}
		post, ok := sel.Posterior(p.Arm)
		if !ok {
			t.Log("Posterior returned ok=false for registered arm")
			return false
		}
		if post.Alpha() != float64(s+1) {
			t.Logf("alpha = %v, want %d", post.Alpha(), s+1)
			return false
		}
		if post.Beta() != float64(fCount+1) {
			t.Logf("beta = %v, want %d", post.Beta(), fCount+1)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_PosteriorMeanInUnit asserts that the posterior
// mean is always in the closed interval [0, 1] no matter how
// many successes or failures have been observed. The mean falls
// out of alpha/(alpha+beta) and the prior is Beta(1, 1), so the
// interval is the natural range of any Beta mean.
func TestProperty_PosteriorMeanInUnit(t *testing.T) {
	t.Parallel()
	f := func(p arbArmPair) bool {
		if p.Arm == "" {
			return true
		}
		sel := NewSelector([]string{p.Arm})
		for _, ok := range p.Success {
			if err := sel.Observe(p.Arm, ok); err != nil {
				t.Logf("Observe: %v", err)
				return false
			}
		}
		m, err := sel.PosteriorMean(p.Arm)
		if err != nil {
			t.Logf("PosteriorMean: %v", err)
			return false
		}
		if m < 0 || m > 1 {
			t.Logf("mean = %v, out of [0,1]", m)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_UnknownArmRejected asserts that Observe and
// PosteriorMean on an arm that was never registered always
// return ErrUnknownArm. The previous implementation accepted
// silent no-ops; the typed return is the contract.
func TestProperty_UnknownArmRejected(t *testing.T) {
	t.Parallel()
	f := func(p arbArmPair) bool {
		// Only assert "the arm is unknown" when the random arm
		// is not the one registered. The shrinker can drive Arm
		// to "", which is harmless.
		sel := NewSelector([]string{"known"})
		if p.Arm == "known" {
			return true
		}
		if err := sel.Observe(p.Arm, true); err != ErrUnknownArm {
			t.Logf("Observe(unknown) = %v, want ErrUnknownArm", err)
			return false
		}
		if _, err := sel.PosteriorMean(p.Arm); err != ErrUnknownArm {
			t.Logf("PosteriorMean(unknown) = %v, want ErrUnknownArm", err)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// arbArmSet returns a generator that produces a non-empty set of
// distinct arm IDs. The selector under test must not be empty
// (Select returns ErrNoArms) so we always include at least one.
type arbArmSet struct {
	IDs []string
}

func (arbArmSet) Generate(rng *rand.Rand, _ int) reflect.Value {
	n := 1 + rng.Intn(5) // 1..5 arms is plenty for property testing
	seen := make(map[string]struct{}, n)
	ids := make([]string, 0, n)
	for len(ids) < n {
		id := string([]byte{
			byte('a' + rng.Intn(26)),
			byte('a' + rng.Intn(26)),
			byte('a' + rng.Intn(26)),
		})
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return reflect.ValueOf(arbArmSet{IDs: ids})
}

// TestProperty_SelectWithRNGReturnsRegisteredArm asserts that
// every arm a fresh SelectWithRNG returns is one of the arms
// the selector was constructed with. The selector's order list
// is the source of truth for the registered set.
func TestProperty_SelectWithRNGReturnsRegisteredArm(t *testing.T) {
	t.Parallel()
	f := func(s arbArmSet, seed uint64) bool {
		if len(s.IDs) == 0 {
			return true
		}
		sel := NewSelector(s.IDs)
		rng := randv2.New(randv2.NewPCG(seed, seed))
		registered := make(map[string]struct{}, len(s.IDs))
		for _, id := range s.IDs {
			registered[id] = struct{}{}
		}
		for i := 0; i < 16; i++ {
			arm, err := sel.SelectWithRNG(rng)
			if err != nil {
				t.Logf("SelectWithRNG: %v", err)
				return false
			}
			if _, ok := registered[arm]; !ok {
				t.Logf("got unregistered arm %q", arm)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_SelectWithRNGDeterministicSameSeed asserts that
// two Selectors built with the same arm IDs and the same RNG
// seed produce the same sequence of selections. This is the
// pinned version of the determinism contract that the package
// comment promises; running the same seed twice must yield the
// same arm.
func TestProperty_SelectWithRNGDeterministicSameSeed(t *testing.T) {
	t.Parallel()
	f := func(s arbArmSet, seed uint64) bool {
		if len(s.IDs) == 0 {
			return true
		}
		selA := NewSelector(s.IDs)
		selB := NewSelector(s.IDs)
		rngA := randv2.New(randv2.NewPCG(seed, seed))
		rngB := randv2.New(randv2.NewPCG(seed, seed))
		for i := 0; i < 32; i++ {
			a, errA := selA.SelectWithRNG(rngA)
			b, errB := selB.SelectWithRNG(rngB)
			if errA != nil || errB != nil {
				t.Logf("errA=%v errB=%v", errA, errB)
				return false
			}
			if a != b {
				t.Logf("seed %d step %d: %q vs %q", seed, i, a, b)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_ThompsonSamplingConvergesToWinner asserts that,
// after many successes on one arm and many failures on the
// others, the better arm is selected the majority of the time.
// We sample many seeds so the assertion is a probabilistic
// invariant, not a single-seed quirk.
func TestProperty_ThompsonSamplingConvergesToWinner(t *testing.T) {
	t.Parallel()
	armIDs := []string{"winner", "loser-a", "loser-b", "loser-c"}
	f := func(seed uint64) bool {
		sel := NewSelector(armIDs)
		for i := 0; i < 30; i++ {
			if err := sel.Observe("winner", true); err != nil {
				t.Fatalf("Observe winner: %v", err)
			}
		}
		for _, id := range []string{"loser-a", "loser-b", "loser-c"} {
			for i := 0; i < 30; i++ {
				if err := sel.Observe(id, false); err != nil {
					t.Fatalf("Observe %s: %v", id, err)
				}
			}
		}
		rng := randv2.New(randv2.NewPCG(seed, seed))
		counts := map[string]int{}
		for i := 0; i < 200; i++ {
			arm, err := sel.SelectWithRNG(rng)
			if err != nil {
				t.Fatalf("SelectWithRNG: %v", err)
			}
			counts[arm]++
		}
		// The winner should be selected at least as many times as
		// any single loser. The exact figure depends on the seed,
		// but the lower bound "winner >= any loser" is robust.
		winCount := counts["winner"]
		for _, id := range []string{"loser-a", "loser-b", "loser-c"} {
			if winCount < counts[id] {
				t.Logf("seed %d: winner=%d < %s=%d", seed, winCount, id, counts[id])
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 20}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_OrderIsPreservedAfterObservations asserts that
// Observe never mutates the registered arm order. Order is the
// source of the determinism contract, so any reshuffle would
// break the regression net above.
func TestProperty_OrderIsPreservedAfterObservations(t *testing.T) {
	t.Parallel()
	f := func(s arbArmSet, seed uint64) bool {
		if len(s.IDs) == 0 {
			return true
		}
		sel := NewSelector(s.IDs)
		want := append([]string(nil), s.IDs...)
		got := append([]string(nil), sel.order...)
		if !sliceEqualUnordered(want, got) {
			return false
		}
		// Drive a few observations so the field is dirty.
		rng := randv2.New(randv2.NewPCG(seed, seed))
		for i := 0; i < 16; i++ {
			id := sel.order[rng.IntN(len(sel.order))]
			ok := rng.IntN(2) == 0
			if err := sel.Observe(id, ok); err != nil {
				return false
			}
		}
		got2 := append([]string(nil), sel.order...)
		return sliceEqualUnordered(want, got2)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_SelectReturnsRegisteredArm asserts that the
// production Select() path returns arms from the registered
// set. The previous implementation rebuilt a fresh PCG on
// every call from time.Now(); the new path uses the
// per-selector RNG and shares the same "registered arm"
// invariant as SelectWithRNG.
func TestProperty_SelectReturnsRegisteredArm(t *testing.T) {
	t.Parallel()
	f := func(s arbArmSet, seed uint64) bool {
		if len(s.IDs) == 0 {
			return true
		}
		// Build a selector and seed its RNG deterministically
		// via NewSelectorWithRNG so the property is repeatable.
		sel := NewSelectorWithRNG(s.IDs, randv2.New(randv2.NewPCG(seed, seed)))
		registered := make(map[string]struct{}, len(s.IDs))
		for _, id := range s.IDs {
			registered[id] = struct{}{}
		}
		for i := 0; i < 16; i++ {
			arm, err := sel.Select()
			if err != nil {
				t.Logf("Select: %v", err)
				return false
			}
			if _, ok := registered[arm]; !ok {
				t.Logf("got unregistered arm %q", arm)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

func sliceEqualUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
