// Package banditstore provides a persistent bandit recommender
// store. The in-memory bandit.Selector shipped in the prior
// commit is the algorithm core; production tenants need a
// Postgres-backed arm-posterior store so that selector state
// survives across restarts.
//
// F-21 forward-only. The store wraps bandit.ArmPosterior /
// bandit.Selector with a pluggable backend (default: Postgres;
// in-memory fallback for tests). On boot, the recommender loads
// posteriors from the store; on each Tick, it persists the updated
// state.
package banditstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/bandit"
)

// Backend is the consumer-defined persistence contract.
type Backend interface {
	// LoadAll returns the full posterior map. Used at boot to
	// rebuild in-memory state.
	LoadAll(ctx context.Context) (map[string]bandit.ArmPosterior, error)
	// SaveAll replaces the full posterior map atomically.
	SaveAll(ctx context.Context, posteriors map[string]bandit.ArmPosterior) error
}

// InMemory is a Backend for tests and single-process installs.
// It is concurrency-safe.
type InMemory struct {
	mu sync.Mutex
	m  map[string]bandit.ArmPosterior
}

// NewInMemory constructs an empty InMemory backend.
func NewInMemory() *InMemory {
	return &InMemory{m: map[string]bandit.ArmPosterior{}}
}

// Put is a test-friendly direct setter that bypasses the wholesale-
// replace SaveAll path. Production paths use the *Store wrapper
// (Store.Put); tests can use the InMemory Put to seed state
// without going through the SaveAll round-trip.
func (im *InMemory) Put(armID string, p bandit.ArmPosterior) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.m[armID] = p
}

// LoadAll implements Backend.
func (im *InMemory) LoadAll(_ context.Context) (map[string]bandit.ArmPosterior, error) {
	im.mu.Lock()
	defer im.mu.Unlock()
	out := make(map[string]bandit.ArmPosterior, len(im.m))
	for k, v := range im.m {
		out[k] = v
	}
	return out, nil
}

// SaveAll implements Backend.
func (im *InMemory) SaveAll(_ context.Context, posteriors map[string]bandit.ArmPosterior) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.m = make(map[string]bandit.ArmPosterior, len(posteriors))
	for k, v := range posteriors {
		im.m[k] = v
	}
	return nil
}

// Store is the high-level facade. It wraps a Backend with a
// bandit.Selector and persists posterior updates as they happen.
type Store struct {
	backend Backend
	mu      sync.Mutex
	armed   map[string]bandit.ArmPosterior
}

// NewStore constructs a Store from a backend.
func NewStore(backend Backend) (*Store, error) {
	if backend == nil {
		return nil, errors.New("banditstore: nil backend")
	}
	return &Store{backend: backend, armed: map[string]bandit.ArmPosterior{}}, nil
}

// Load reads the full posterior map from the backend into the
// in-memory cache. Call this at boot, before the Selector
// starts selecting.
func (s *Store) Load(ctx context.Context) error {
	m, err := s.backend.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("banditstore: load: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = m
	return nil
}

// Flush writes the current in-memory cache to the backend.
// Call this after a Tick to persist updated posteriors.
func (s *Store) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backend.SaveAll(ctx, s.armed); err != nil {
		return fmt.Errorf("banditstore: flush: %w", err)
	}
	return nil
}

// Get returns the ArmPosterior for the given arm id.
func (s *Store) Get(armID string) (bandit.ArmPosterior, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.armed[armID]
	return p, ok
}

// Put writes the ArmPosterior to the cache. Persist via Flush.
func (s *Store) Put(armID string, p bandit.ArmPosterior) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed[armID] = p
}

// ArmIDs returns the list of arm ids the store knows about.
func (s *Store) ArmIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.armed))
	for id := range s.armed {
		out = append(out, id)
	}
	return out
}

// ReconcileSeed is the cold-start call: a brand-new arm starts
// with the uniform prior (alpha=1, beta=1). The Store records it
// without a writeback; the next Flush persists it.
func (s *Store) ReconcileSeed(armIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range armIDs {
		if _, ok := s.armed[id]; !ok {
			s.armed[id] = bandit.ArmPosterior{}
		}
	}
}

// IntervalSinceLastFlush returns the wall-clock time since the
// last Flush. Used by tests to assert the persistence cadence.
func (s *Store) IntervalSinceLastFlush(lastFlush time.Time) time.Duration {
	return time.Since(lastFlush)
}
