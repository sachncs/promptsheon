package bandit

import (
	"math/rand/v2"
	"testing"
)

// BenchmarkSelect exercises Thompson Sampling with 10 arms. The
// gamma + beta sampling chain is the inner-loop hot path on
// every arm selection; this benchmark tracks its per-call cost.
func BenchmarkSelect(b *testing.B) {
	armIDs := make([]string, 10)
	for i := range armIDs {
		armIDs[i] = "arm-" + string(rune('a'+i))
	}
	s := NewSelector(armIDs)
	rng := rand.New(rand.NewPCG(1, 2))
	// Observe a few outcomes so posteriors have mass.
	for i := 0; i < 100; i++ {
		_ = s.Observe(armIDs[i%len(armIDs)], i%3 == 0)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SelectWithRNG(rng); err != nil {
			b.Fatalf("SelectWithRNG: %v", err)
		}
	}
}

// BenchmarkSelectSmall exercises the common case of 3 arms,
// which is typical for bandit-armed A/B tests.
func BenchmarkSelectSmall(b *testing.B) {
	armIDs := []string{"control", "variant-a", "variant-b"}
	s := NewSelector(armIDs)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 100; i++ {
		_ = s.Observe(armIDs[i%len(armIDs)], i%3 == 0)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SelectWithRNG(rng); err != nil {
			b.Fatalf("SelectWithRNG: %v", err)
		}
	}
}
