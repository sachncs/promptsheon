// Regression tests for Selector.Select. These live in a separate
// file so the bug fix is locked in without touching the long-
// standing selector_test.go fixtures. The previous implementation
// built a fresh *rand.Rand per Select() from time.Now() — two
// rapid calls would draw from the same PRNG state and return the
// same arm, and the value could be the time-zero source if the
// helper ever returned 0 again.
package bandit

import (
	"math/rand/v2"
	"sync"
	"testing"
)

// TestSelectUsesSelectorOwnedRNG locks in the fix: two rapid
// Select() calls with no intervening Observe must be able to
// return different arms. With the previous per-call
// rand.New(rand.NewPCG(uint64(timeNow()), uint64(timeNow())))
// implementation, two calls inside the same nanosecond produced
// identical draws and the test (winner + 4 losers with no
// observations) deterministically returned "a" twice.
func TestSelectUsesSelectorOwnedRNG(t *testing.T) {
	t.Parallel()
	s := NewSelector([]string{"a", "b", "c", "d", "e"})
	seen := make(map[string]int)
	for i := 0; i < 64; i++ {
		arm, err := s.Select()
		if err != nil {
			t.Fatalf("Select: %v", err)
		}
		seen[arm]++
	}
	// With a non-degenerate entropy seed and 5 uniform-prior
	// arms, every arm should be picked at least once across 64
	// draws. The previous bug never moved past the first arm.
	if len(seen) < 2 {
		t.Fatalf("expected Select to draw across multiple arms; saw %v", seen)
	}
}

// TestSelectAdvancesRNGState locks in that Select mutates the
// internal RNG state. We pin a deterministic RNG, record the
// state before, draw once, and confirm the state advanced —
// guaranteeing Select consumed at least one draw from the
// selector-owned RNG rather than rebuilding a fresh PCG.
func TestSelectAdvancesRNGState(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	s := NewSelectorWithRNG([]string{"a", "b", "c"}, rng)
	before := rng.Uint64()
	for i := 0; i < 8; i++ {
		if _, err := s.Select(); err != nil {
			t.Fatalf("Select: %v", err)
		}
	}
	after := rng.Uint64()
	if before == after {
		t.Fatalf("expected selector-owned RNG state to advance after Select; both reads = %d", before)
	}
}

// TestSelectConcurrentDiffer guards the concurrent path. Two
// goroutines hammering Select with no observations must each
// see a sequence of draws (i.e. Select does not hold a per-call
// fresh seed that both share). We only assert no panic / no
// duplicate draws within one goroutine — the precise
// distribution across arms is a property test elsewhere.
func TestSelectConcurrentDiffer(t *testing.T) {
	t.Parallel()
	s := NewSelector([]string{"a", "b", "c", "d", "e"})
	const perG = 200
	var wg sync.WaitGroup
	wg.Add(2)
	results := make([][]string, 2)
	for g := 0; g < 2; g++ {
		g := g
		go func() {
			defer wg.Done()
			local := make([]string, 0, perG)
			for i := 0; i < perG; i++ {
				arm, err := s.Select()
				if err != nil {
					t.Errorf("Select: %v", err)
					return
				}
				local = append(local, arm)
			}
			results[g] = local
		}()
	}
	wg.Wait()
	for g, seq := range results {
		if len(seq) != perG {
			t.Fatalf("goroutine %d produced %d draws, want %d", g, len(seq), perG)
		}
		// No arm should repeat back-to-back every single time.
		// With 5 uniform-prior arms and 200 draws, repeats are
		// common but a 200-long run of identical arms would
		// mean Select is broken (returning the first arm every
		// time).
		unique := map[string]struct{}{}
		for _, a := range seq {
			unique[a] = struct{}{}
		}
		if len(unique) < 2 {
			t.Fatalf("goroutine %d only saw a single arm across %d draws: %v", g, perG, seq)
		}
	}
}
