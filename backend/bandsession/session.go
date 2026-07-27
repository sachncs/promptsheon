// Package bandsession couples a bandit.Selector to a persistent
// banditstore.Store and exposes the load/select/observe/flush
// lifecycle used by the production recommender.
//
// The Session is the production path for the bandit recommender:
// load on init, select on demand, observe outcomes, flush on
// demand. The Store backend may be either the InMemory backend
// (default; tests; single-process installs) or the SQLite
// backend (production) — the Session contract is identical
// for both.
package bandsession

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/bandit"
	"github.com/sachncs/promptsheon/internal/banditstore"
)

type selectionObserver interface {
	RecordBanditSelection(string)
}

// Session wraps a bandit.Selector and a banditstore.Store with the
// lifecycle that production needs: load on init, observe on
// outcome, persist on Flush.
type Session struct {
	store     *banditstore.Store
	selector  *bandit.Selector
	mu        sync.Mutex
	loaded    bool
	armIDs    []string
	rngSeed   uint64
	lastFlush time.Time
	runID     string
	observer  selectionObserver
}

// New constructs a Session around the given store. rngSeed is the
// seed for the underlying Selector's RNG; zero means "seed by
// wall-clock nanos". For tests, set rngSeed to a known value
// (e.g. 42) for deterministic arm selection.
func New(store *banditstore.Store, rngSeed uint64) (*Session, error) {
	if store == nil {
		return nil, fmt.Errorf("banditsession: nil store")
	}
	var id [16]byte
	if _, err := cryptorand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("banditsession: run id: %w", err)
	}
	return &Session{
		store:   store,
		rngSeed: rngSeed,
		runID:   hex.EncodeToString(id[:]),
	}, nil
}

// Load reads the persisted counters from the store and feeds
// them into the Selector. Call this at boot, before any Select.
// The store may be empty; cold-start arms are seeded with the
// uniform Beta(1, 1) prior (alpha=1, beta=1).
func (s *Session) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("banditsession: load: %w", err)
	}
	armIDs := make([]string, 0, len(state))
	for id := range state {
		armIDs = append(armIDs, id)
	}
	s.armIDs = armIDs
	s.selector = s.buildSelector(armIDs)
	// Seed the Selector's posterior for every known arm from the
	// merged store state. The store's Counter has the per-arm
	// successes/failures; the Selector's posterior needs the
	// derived (alpha=1+s, beta=1+f).
	for id, c := range state {
		post := bandit.NewArmPosteriorWithCounts(1+float64(c.Successes), 1+float64(c.Failures))
		if err := s.selector.SetPosterior(id, *post); err != nil && err != bandit.ErrUnknownArm {
			// armIDs list and Selector order must agree; if
			// they don't the Selector's constructor didn't
			// see this arm, which is a programmer error.
			return fmt.Errorf("banditsession: seed posterior %q: %w", id, err)
		}
	}
	s.loaded = true
	s.lastFlush = time.Now().UTC()
	return nil
}

// RegisterArms adds new arm IDs that did not exist in the store
// at Load time. The new arms are seeded with the uniform prior.
// RegisterArms must NOT invent observations: the underlying
// banditstore.Store.RegisterArms writes a (0, 0) Counter for
// each arm, which translates to alpha=1, beta=1 once the
// Selector is rebuilt.
func (s *Session) RegisterArms(ctx context.Context, armIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return fmt.Errorf("banditsession: must Load before RegisterArms")
	}
	if err := s.store.RegisterArms(ctx, armIDs); err != nil {
		return fmt.Errorf("banditsession: register: %w", err)
	}
	if err := s.store.Flush(ctx); err != nil {
		return fmt.Errorf("banditsession: flush: %w", err)
	}
	s.lastFlush = time.Now().UTC()
	// Reload the in-memory selector with the new arm list.
	state, err := s.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("banditsession: reload: %w", err)
	}
	allIDs := make([]string, 0, len(state))
	for id := range state {
		allIDs = append(allIDs, id)
	}
	s.armIDs = allIDs
	s.selector = s.buildSelector(allIDs)
	for id, c := range state {
		post := bandit.NewArmPosteriorWithCounts(1+float64(c.Successes), 1+float64(c.Failures))
		if err := s.selector.SetPosterior(id, *post); err != nil && err != bandit.ErrUnknownArm {
			return fmt.Errorf("banditsession: seed posterior %q: %w", id, err)
		}
	}
	return nil
}

// Select returns the arm with the highest Thompson sample. The
// result is non-deterministic unless the Session was constructed
// with a non-zero rngSeed.
func (s *Session) Select() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selector == nil {
		return "", fmt.Errorf("banditsession: selector not loaded")
	}
	arm, err := s.selector.Select()
	if err != nil {
		return "", fmt.Errorf("banditsession: select: %w", err)
	}
	if s.observer != nil {
		s.observer.RecordBanditSelection(s.runID)
	}
	return arm, nil
}

// RunID identifies this process-local selection session.
func (s *Session) RunID() string { return s.runID }

// SetSelectionObserver bridges selections to an observability collector.
func (s *Session) SetSelectionObserver(observer selectionObserver) { s.observer = observer }

// Observe records one outcome (success or failure) for the
// given arm. The order is: validate arm, persist to store,
// then mutate selector. A failing backend must leave the
// selector's posterior unchanged — the selector is the
// in-memory cache and only advances once the backend has
// accepted the write, so a transient failure does not poison
// the next arm draw. The selector and the store stay
// coherent because both grow monotonically on every Observe.
func (s *Session) Observe(ctx context.Context, armID string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selector == nil {
		return fmt.Errorf("banditsession: selector not loaded")
	}
	// Validate the arm against the registered set before any
	// backend I/O. ErrUnknownArm is the canonical signal; the
	// selector is the source of truth for which arms the
	// session has accepted.
	if _, ok := s.selector.Posterior(armID); !ok {
		return fmt.Errorf("banditsession: selector observe: %w", bandit.ErrUnknownArm)
	}
	if err := s.store.Observe(ctx, armID, success); err != nil {
		return fmt.Errorf("banditsession: store observe: %w", err)
	}
	if err := s.store.Flush(ctx); err != nil {
		return fmt.Errorf("banditsession: flush: %w", err)
	}
	// Selector mutation only after the backend has accepted
	// the write. A backend failure leaves the posterior
	// untouched so the next Select() draws from the same
	// state as before.
	if err := s.selector.Observe(armID, success); err != nil {
		return fmt.Errorf("banditsession: selector observe: %w", err)
	}
	s.lastFlush = time.Now().UTC()
	return nil
}

// Flush writes the in-memory store state to the backend. With
// the SQLite backend this is the commit boundary; with the
// InMemory backend it is a no-op.
func (s *Session) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.Flush(ctx); err != nil {
		return fmt.Errorf("banditsession: flush: %w", err)
	}
	s.lastFlush = time.Now().UTC()
	return nil
}

// ArmIDs returns the list of arms the session currently knows
// about.
func (s *Session) ArmIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.armIDs))
	copy(out, s.armIDs)
	return out
}

// PosteriorMean returns the current Mean of the given arm's
// posterior. Used by the observability surface to report the
// per-arm confidence interval.
func (s *Session) PosteriorMean(armID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selector == nil {
		return 0, fmt.Errorf("banditsession: selector not loaded")
	}
	return s.selector.PosteriorMean(armID)
}

// Close flushes the session and releases the store. Idempotent.
func (s *Session) Close(ctx context.Context) error {
	return s.Flush(ctx)
}

// LastFlush returns the timestamp of the most recent successful
// Flush. The zero value is returned until Load or Flush has run.
func (s *Session) LastFlush() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastFlush
}

// buildSelector constructs a fresh Selector seeded by the
// configured rngSeed. The Selector's per-arm posteriors are
// seeded by Load/RegisterArms after construction, so
// buildSelector itself only wires the RNG and the arm order.
func (s *Session) buildSelector(armIDs []string) *bandit.Selector {
	var seed [32]byte
	if s.rngSeed != 0 {
		// Reseed from a known value for deterministic tests.
		for i := 0; i < 8; i++ {
			seed[i] = byte(s.rngSeed >> (i * 8))
		}
	} else {
		// Seed from the wall clock for production.
		now := time.Now().UnixNano()
		for i := 0; i < 8; i++ {
			seed[i] = byte(now >> (i * 8))
		}
	}
	rng := rand.New(rand.NewPCG(
		// #nosec G404 -- we want a non-cryptographic source for
		// Thompson Sampling arm selection; the PRNG state is
		// derived from a wall clock or a caller-supplied seed.
		uint64(seed[0])<<56|uint64(seed[1])<<48|uint64(seed[2])<<40|uint64(seed[3])<<32|uint64(seed[4])<<24|uint64(seed[5])<<16|uint64(seed[6])<<8|uint64(seed[7]),
		uint64(seed[8])<<56|uint64(seed[9])<<48|uint64(seed[10])<<40|uint64(seed[11])<<32|uint64(seed[12])<<24|uint64(seed[13])<<16|uint64(seed[14])<<8|uint64(seed[15]),
	))
	return bandit.NewSelectorWithRNG(armIDs, rng)
}
