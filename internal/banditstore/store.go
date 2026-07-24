// Package banditstore provides a CRDT-backed persistent store
// for the bandit recommender. The historical implementation
// used a wholesale LoadAll/SaveAll snapshot model keyed by
// arm id; that design loses concurrent updates when two
// replicas write the same arm independently. The current
// implementation tracks per-(arm, replica) grow-only
// counters (successes, failures) and merges on observation,
// which converges under concurrent writes without
// coordination.
//
// Concretely:
//
//   - State = map[armID]Counter, where Counter is a
//     grow-only (successes, failures) pair.
//   - Merge = component-wise max within a single (arm, replica)
//     row. The backend's Load Sums per-replica counters so the
//     selector sees the global observation total across replicas.
//   - The backend persists one row per (arm_id, replica_id);
//     conflict-safe merge on the SQL side is just MAX() and
//     Load is SUM() GROUP BY arm_id.
//   - The Store facade exposes Observe (one trial), Load
//     (full state), and Merge (apply a remote snapshot).
//
// Pure merge helpers and algebraic properties live in
// internal/bandit/crdt.go and crdt_test.go; this file only
// adds the persistence contract.
package banditstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/sachncs/promptsheon/internal/bandit"
)

// Backend is the per-replica persistence contract. A backend
// stores one (successes, failures) tuple per (arm, replica)
// pair and exposes:
//
//   - Load: pull every replica's counters into one State.
//   - Observe: increment one (arm, replica) counter.
//   - Merge: fold a remote State into the local store using
//     the conflict-safe merge operation.
type Backend interface {
	Load(ctx context.Context) (bandit.State, error)
	Observe(ctx context.Context, replicaID, armID string, success bool) error
	Merge(ctx context.Context, replicaID string, remote bandit.State) error
}

// InMemory is a Backend for tests and single-process installs.
// It is concurrency-safe and stores one row per replica so the
// concurrency semantics line up with the SQLite backend — a
// concurrent merge from a remote replica is just component-wise
// max against the local replica's counters.
//
// Load sums per-replica counters: every replica contributes its
// own (successes, failures) tally to the arm's bucket and the
// selector sees the global observation total. The per-replica
// Merge op stays component-wise max because duplicate snapshots
// from the same replica must be no-ops, not additions.
type InMemory struct {
	mu   sync.Mutex
	rows map[string]map[string]bandit.Counter // replica -> arm -> counter
}

// NewInMemory constructs an empty InMemory backend.
func NewInMemory() *InMemory {
	return &InMemory{rows: map[string]map[string]bandit.Counter{}}
}

// Seed is a test-friendly direct setter that records a (replica,
// arm, counter) row without going through Observe/Merge. The
// test fixtures use it to lay down pre-merge state without
// having to drive N observations through the public API.
func (im *InMemory) Seed(replicaID, armID string, c bandit.Counter) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.rows[replicaID] == nil {
		im.rows[replicaID] = map[string]bandit.Counter{}
	}
	im.rows[replicaID][armID] = c
}

// Load implements Backend. The returned State is the per-arm
// SUM across every replica's contribution — every replica's
// observations count once toward the arm's effective posterior.
func (im *InMemory) Load(_ context.Context) (bandit.State, error) {
	im.mu.Lock()
	defer im.mu.Unlock()
	out := bandit.State{}
	for _, armMap := range im.rows {
		for arm, c := range armMap {
			cur := out[arm]
			cur.Successes += c.Successes
			cur.Failures += c.Failures
			out[arm] = cur
		}
	}
	return out, nil
}

// Observe implements Backend. The (replica, arm) counter has
// one coordinate bumped; the rest of the row is untouched.
func (im *InMemory) Observe(_ context.Context, replicaID, armID string, success bool) error {
	if replicaID == "" || armID == "" {
		return errors.New("banditstore: replicaID and armID are required")
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.rows[replicaID] == nil {
		im.rows[replicaID] = map[string]bandit.Counter{}
	}
	cur := im.rows[replicaID][armID]
	if success {
		cur.Successes++
	} else {
		cur.Failures++
	}
	im.rows[replicaID][armID] = cur
	return nil
}

// Merge implements Backend. The remote State is treated as a
// snapshot from `replicaID`; per-arm counters are component-wise
// maxed with the local replica's counters. This is the
// grow-only CRDT merge.
func (im *InMemory) Merge(_ context.Context, replicaID string, remote bandit.State) error {
	if replicaID == "" {
		return errors.New("banditstore: replicaID required for merge")
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.rows[replicaID] == nil {
		im.rows[replicaID] = map[string]bandit.Counter{}
	}
	for arm, c := range remote {
		merged := bandit.MergeCounters(im.rows[replicaID][arm], c)
		im.rows[replicaID][arm] = merged
	}
	return nil
}

// Store is the high-level facade. It owns the local replica ID
// and provides the production-path API (Observe, Load, Merge).
type Store struct {
	backend   Backend
	mu        sync.Mutex
	replicaID string
}

// NewStore constructs a Store with an entropy-seeded local
// replica id. Production uses this constructor; tests use
// NewStoreWithReplica for determinism.
func NewStore(backend Backend) (*Store, error) {
	if backend == nil {
		return nil, errors.New("banditstore: nil backend")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("banditstore: replica id: %w", err)
	}
	return &Store{
		backend:   backend,
		replicaID: hex.EncodeToString(id[:]),
	}, nil
}

// NewStoreWithReplica constructs a Store with a caller-chosen
// replica id. The test seam: a property test that compares two
// stores side-by-side needs deterministic replica ids, and
// seeding crypto/rand between tests is brittle.
func NewStoreWithReplica(backend Backend, replicaID string) (*Store, error) {
	if backend == nil {
		return nil, errors.New("banditstore: nil backend")
	}
	if replicaID == "" {
		return nil, errors.New("banditstore: replicaID is required")
	}
	return &Store{backend: backend, replicaID: replicaID}, nil
}

// ReplicaID returns the local replica id used for write-side
// attribution. Exposed so the API/test surfaces can log it.
func (s *Store) ReplicaID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replicaID
}

// SetReplicaID replaces the local replica id. Tests use it
// after construction to assert specific merge paths; production
// never calls it.
func (s *Store) SetReplicaID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replicaID = id
}

// Load reads every (arm, replica) row from the backend and
// folds them into one State via per-arm SUM across replicas.
// Duplicate snapshots from the same replica stay idempotent
// because the per-replica Merge is component-wise max; the
// cross-replica aggregation sums distinct observations.
func (s *Store) Load(ctx context.Context) (bandit.State, error) {
	out, err := s.backend.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("banditstore: load: %w", err)
	}
	return out, nil
}

// Observe records one trial outcome (success or failure) for
// the given arm. The observation is attributed to the local
// replica id and persisted as a per-(arm, replica) bump; no
// other replica is touched. The local merge is conflict-safe
// even if a concurrent Observe on the same arm raced ahead
// because the merge semantics are component-wise max.
func (s *Store) Observe(ctx context.Context, armID string, success bool) error {
	s.mu.Lock()
	replicaID := s.replicaID
	s.mu.Unlock()
	if err := s.backend.Observe(ctx, replicaID, armID, success); err != nil {
		return fmt.Errorf("banditstore: observe %q: %w", armID, err)
	}
	return nil
}

// Merge folds a remote replica's State into the local store.
// The merge is the per-arm component-wise max; concurrent
// merges of disjoint arms or disjoint replicas converge to the
// same State without coordination.
func (s *Store) Merge(ctx context.Context, replicaID string, remote bandit.State) error {
	if remote == nil {
		return nil
	}
	if err := s.backend.Merge(ctx, replicaID, remote); err != nil {
		return fmt.Errorf("banditstore: merge %q: %w", replicaID, err)
	}
	return nil
}

// RegisterArms records new arm IDs in the local store with
// the uniform Beta(1,1) prior (successes=0, failures=0).
// RegisterArms must NOT invent observations — a registration
// with non-zero counters would silently bias the bandit
// toward that arm.
func (s *Store) RegisterArms(ctx context.Context, armIDs []string) error {
	for _, id := range armIDs {
		if id == "" {
			continue
		}
		// Counter{} == {0, 0}: the merge is a no-op on the
		// SQL side but the row now exists for the (replica,
		// arm) pair. The store layer treats absence and
		// zero-counter rows the same way (the effective
		// posterior is alpha=1, beta=1 either way).
		if err := s.backend.Merge(ctx, s.replicaIDFor(ctx), bandit.State{id: {}}); err != nil {
			return fmt.Errorf("banditstore: register %q: %w", id, err)
		}
	}
	return nil
}

// Flush is a no-op for the in-memory backend; the SQLite
// backend uses it as the commit boundary. Provided so the
// bandsession.Session contract (load / select / observe /
// flush) still composes with the new Store API.
func (s *Store) Flush(_ context.Context) error { return nil }

// ArmIDs returns the set of arms known to the local store.
// Used by bandsession to rebuild its in-memory Selector after
// a cold start.
func (s *Store) ArmIDs(ctx context.Context) ([]string, error) {
	st, err := s.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(st))
	for id := range st {
		out = append(out, id)
	}
	return out, nil
}

// Get returns the merged (across all replicas) Counter for the
// given arm, plus a bool indicating presence. bandsession uses
// it to seed the in-memory Selector on Load.
func (s *Store) Get(ctx context.Context, armID string) (bandit.Counter, bool) {
	st, err := s.Load(ctx)
	if err != nil {
		return bandit.Counter{}, false
	}
	c, ok := st[armID]
	return c, ok
}

// replicaIDFor returns the local replica id under the store
// mutex. The helper exists so RegisterArms can call the
// backend without taking the lock twice.
func (s *Store) replicaIDFor(_ context.Context) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replicaID
}
