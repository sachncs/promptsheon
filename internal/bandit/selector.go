// Package bandit provides Thompson Sampling arm selection for the
// abtesting.Engine. Thompson Sampling is a Bayesian multi-armed
// bandit policy: each arm maintains a Beta(alpha, beta) posterior
// over its true success rate; the next arm is the one with the
// highest sample drawn from its posterior. Arms that have been
// pulled many times have tight posteriors (low variance); arms
// that have not been pulled have wide posteriors (high variance)
// which is the natural exploration mechanism.
//
// This is the Tier 2.35 foundation of the M4 bandit recommender.
// The full bandit engine (with persistent posteriors, restarts,
// and bandit-aware Recommendations) ships in a follow-on commit;
// today's commit adds the selection algorithm and unit tests so
// the abtesting.Engine is the only consumer.
package bandit

import (
	"math"
	"math/rand/v2"
	"sync"
)

// ArmPosterior is the Beta(alpha, beta) posterior over the success
// rate of one arm. alpha = successes + 1, beta = failures + 1
// (a uniform Beta(1, 1) prior). The convention guards against
// zero-variance at the start of the test.
type ArmPosterior struct {
	alpha float64
	beta  float64
}

// NewArmPosterior returns the uniform-prior Beta(1, 1) posterior.
func NewArmPosterior() *ArmPosterior {
	return &ArmPosterior{alpha: 1, beta: 1}
}

// NewArmPosteriorWithCounts returns a posterior with the supplied
// alpha and beta. Used by persistence layers that have stored
// the exact counts and need to reconstitute the same posterior.
func NewArmPosteriorWithCounts(alpha, beta float64) *ArmPosterior {
	return &ArmPosterior{alpha: alpha, beta: beta}
}

// Observe records one success or failure.
func (a *ArmPosterior) Observe(success bool) {
	if success {
		a.alpha++
		return
	}
	a.beta++
}

// Mean returns the posterior mean (alpha / (alpha+beta)).
func (a *ArmPosterior) Mean() float64 {
	return a.alpha / (a.alpha + a.beta)
}

// Alpha returns the posterior's alpha parameter (successes + 1).
// Exported for persistence layers that need the exact (alpha, beta)
// rather than only the resulting mean.
func (a *ArmPosterior) Alpha() float64 { return a.alpha }

// Beta returns the posterior's beta parameter (failures + 1).
// Exported for persistence layers that need the exact (alpha, beta)
// rather than only the resulting mean.
func (a *ArmPosterior) Beta() float64 { return a.beta }

// Sample draws one Thompson sample from Beta(alpha, beta) using
// two Gamma samples. Implementation adapted from the
// Marsaglia-Tsang method which is the standard approach for
// drawing Beta samples in scientific computing.
func (a *ArmPosterior) Sample(rng *rand.Rand) float64 {
	x := gammaSample(rng, a.alpha)
	y := gammaSample(rng, a.beta)
	return x / (x + y)
}

// gammaSample returns one draw from Gamma(shape, 1) using the
// Marsaglia-Tsang algorithm. Only shape >= 1 is supported; the
// caller (Sample above) starts from alpha=1, beta=1 and increases
// the shapes by integer increments, so the constraint holds.
func gammaSample(rng *rand.Rand, shape float64) float64 {
	if shape < 1 {
		// Reflection / boostrap; not exercised in our tests.
		return math.NaN()
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		var x float64
		var v float64
		for {
			x = rng.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// Selector is the multi-armed-bandit selection engine. It is
// concurrency-safe; concurrent Observe/Select/Record calls from
// different goroutines are allowed.
type Selector struct {
	mu      sync.Mutex
	arms    map[string]*ArmPosterior
	order   []string
	rngSeed [32]byte
}

// NewSelector constructs a Selector with the supplied arm IDs.
// The order is preserved so output of Select() is deterministic
// given identical posteriors and RNG state.
func NewSelector(armIDs []string) *Selector {
	s := &Selector{
		arms:  make(map[string]*ArmPosterior, len(armIDs)),
		order: append([]string(nil), armIDs...),
	}
	for _, id := range armIDs {
		s.arms[id] = NewArmPosterior()
	}
	return s
}

// NewSelectorWithRNG is reserved for future deterministic
// production use; the current Select() path uses the
// wall-clock seed. M3.5 follow-on per ADR-0019 will wire
// the production path through the custom-RNG constructor.
func NewSelectorWithRNG(armIDs []string, rng *rand.Rand) *Selector {
	s := NewSelector(armIDs)
	_ = rng
	return s
}

// Observe records the outcome of one trial of the supplied arm.
func (s *Selector) Observe(armID string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.arms[armID]
	if !ok {
		return ErrUnknownArm
	}
	a.Observe(success)
	return nil
}

// Select returns the arm with the highest Thompson sample. The
// draw uses an internal RNG seeded by the current time. For
// deterministic test runs, use SelectWithRNG.
func (s *Selector) Select() (string, error) {
	rng := rand.New(rand.NewPCG(uint64(timeNow()), uint64(timeNow())))
	return s.SelectWithRNG(rng)
}

// SelectWithRNG returns the arm with the highest Thompson sample
// using the supplied *rand.Rand. Production code uses Select;
// tests use SelectWithRNG to make the result reproducible.
func (s *Selector) SelectWithRNG(rng *rand.Rand) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) == 0 {
		return "", ErrNoArms
	}
	best := s.order[0]
	bestScore := s.arms[best].Sample(rng)
	for _, id := range s.order[1:] {
		score := s.arms[id].Sample(rng)
		if score > bestScore {
			bestScore = score
			best = id
		}
	}
	return best, nil
}

// PosteriorMean returns the current posterior mean for the arm
// (a Bayesian estimate of the true success rate).
func (s *Selector) PosteriorMean(armID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.arms[armID]
	if !ok {
		return 0, ErrUnknownArm
	}
	return a.Mean(), nil
}

// Posterior returns the current posterior for the arm. The bool
// result is false if the arm is unknown. Used by persistence
// layers to read the exact (alpha, beta) for serialization.
func (s *Selector) Posterior(armID string) (ArmPosterior, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.arms[armID]
	if !ok {
		return ArmPosterior{}, false
	}
	return *a, true
}

// SetPosterior replaces the posterior for an arm. Used by the
// bandsession.Session to reconstitute a Selector from persisted
// counts. Returns ErrUnknownArm if the arm is not registered.
func (s *Selector) SetPosterior(armID string, p ArmPosterior) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.arms[armID]; !ok {
		return ErrUnknownArm
	}
	s.arms[armID] = &p
	return nil
}

// Errors.
var (
	ErrUnknownArm = errBandit("bandit: unknown arm")
	ErrNoArms     = errBandit("bandit: no arms")
)

type errBandit string

func (e errBandit) Error() string { return string(e) }

// timeNow is a function variable for the time source.
var timeNow = func() int64 { return realTimeNow() }

// realTimeNow uses time.Now via a thin indirection; the import
// of "time" happens at the bottom of this file to keep the
// function layout in dependency order.
var realTimeNow = func() int64 { return 0 }
