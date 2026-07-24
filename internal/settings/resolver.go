// Package settings is the operator-tunable runtime config
// layer. The three-layer precedence is:
//
//	hardcoded default < env var / YAML < DB row (system_config)
//
// The DB row is the runtime ceiling. Deleting a row reasserts
// the env-default. PROMPTSHEON_SETTINGS_MODE=env-only disables
// writes (operator can still read). The settings layer is for
// non-secret config (OTel endpoint, sample ratio, insecure flag,
// LLM key *reference* to a provider_keys.id). Secrets go through
// the existing `internal/vault` — vault master keys are NOT
// stored here.
//
// The Resolver is read-on-every-access (not read-once-at-boot)
// so a settings change is reflected immediately. Subscribers
// fire on Set/Delete so dependent pipelines (OTel, LLM
// registry, vault key rotation) can reconfigure without a
// daemon restart. Hot-reload failures are propagated through
// the Resolver and the API handlers return 5xx so the
// operator sees the failed write.
//
// CRDT fields live on the model layer (`models.SystemConfig`);
// the merge operators and algebraic properties are pinned in
// crdt.go / crdt_test.go.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Subscriber is the callback signature a hot-reload notifier
// fires on Set / Delete. Returning an error signals that the
// subscriber could not consume the new value; the Resolver
// returns the error to the caller (typically an HTTP handler)
// so the operator sees the failure mode. Subscribers that
// never fail can `return nil`; subscribers that always fail
// should be retried by the caller rather than blocking the
// write path.
type Subscriber func(value string) error

// Notifier fires callbacks on Set / Delete. Subscribers are
// the OTel and LLM packages; the notifier's callbacks are
// called synchronously after the DB write commits, so a PUT
// returns 200 only when the dependent pipeline has been
// reloaded. If any subscriber returns an error, Publish
// surfaces the first failure and stops invoking later
// subscribers (the system is left in a partially-reloaded
// state — the operator must roll forward).
type Notifier struct {
	mu   sync.Mutex
	subs map[string][]Subscriber
}

// NewNotifier returns a fresh notifier.
func NewNotifier() *Notifier { return &Notifier{subs: map[string][]Subscriber{}} }

// Subscribe registers a callback for a single key. The callback
// fires synchronously on Set(key, value) and Delete(key).
// Multiple subscribers per key are supported; they fire in
// registration order.
func (n *Notifier) Subscribe(key string, fn Subscriber) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.subs[key] = append(n.subs[key], fn)
}

// Publish fires the subscribers for a key. Returns the first
// error any subscriber surfaces. An empty no-op when nobody
// subscribed.
func (n *Notifier) Publish(key, value string) error {
	n.mu.Lock()
	subs := append([]Subscriber{}, n.subs[key]...)
	n.mu.Unlock()
	for _, fn := range subs {
		if err := fn(value); err != nil {
			return err
		}
	}
	return nil
}

// Resolver merges the hardcoded default, env-var, and DB
// sources. Read paths (Get, HasDefault) are safe for
// concurrent use; writes go through Store which is owned by
// the API layer.
type Resolver struct {
	store     Store
	env       EnvSource
	notif     *Notifier
	replicaID string
}

// EnvSource abstracts the env-var lookup so tests can inject
// a static map.
type EnvSource interface {
	Get(key string) (string, bool)
}

// OSEnvSource is the production EnvSource — reads from the OS env.
type OSEnvSource struct{}

// Get returns the env-var value, if any.
func (OSEnvSource) Get(key string) (string, bool) { return os.LookupEnv(key) }

// Store is the interface the Resolver consumes for DB reads +
// writes. The concrete implementation is `*store.SQLite`; the
// interface keeps the package cycle-free.
type Store interface {
	GetSystemConfig(ctx context.Context, key string) (CRDTRecord, error)
	SetSystemConfig(ctx context.Context, rec CRDTRecord) error
	ListSystemConfig(ctx context.Context) ([]CRDTRecord, error)
	MergeSystemConfig(ctx context.Context, replicaID string, records []CRDTRecord) error
}

// SystemConfigRow mirrors models.SystemConfig. Defined here so
// this package doesn't import internal/models (which imports
// the store package — circular).
//
// Deprecated alias for CRDTRecord. Kept so legacy call sites
// that referred to SystemConfigRow do not break; new code
// should use CRDTRecord.
type SystemConfigRow = CRDTRecord

// NewResolver wires the Resolver to its dependencies. env may
// be nil; the Resolver falls back to OSEnvSource. replicaID is
// the per-process CRDT id used to attribute local writes; an
// empty string is treated as "no local replica" and the
// caller must replace it (Resolver.Set will reject writes in
// that mode).
func NewResolver(store Store, notif *Notifier, env EnvSource, replicaID string) *Resolver {
	if env == nil {
		env = OSEnvSource{}
	}
	return &Resolver{store: store, notif: notif, env: env, replicaID: replicaID}
}

// ReplicaID returns the local replica id used to attribute
// Set writes. Useful for observability surfaces.
func (r *Resolver) ReplicaID() string { return r.replicaID }

// Lookup returns the effective record for key while preserving the
// distinction between a missing key, a tombstone, and a live empty value.
// Environment values take precedence over persisted rows, including an
// explicitly empty environment value.
func (r *Resolver) Lookup(ctx context.Context, key string) (CRDTRecord, bool, error) {
	if value, ok := r.env.Get(key); ok {
		return CRDTRecord{Key: key, Value: value}, true, nil
	}
	rec, err := r.store.GetSystemConfig(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return CRDTRecord{}, false, nil
	}
	if err != nil {
		return CRDTRecord{}, false, fmt.Errorf("settings: lookup %q: %w", key, err)
	}
	return rec, true, nil
}

// Get returns the effective value for key.
func (r *Resolver) Get(ctx context.Context, key string) string {
	rec, found, err := r.Lookup(ctx, key)
	if err != nil || !found || rec.Tombstone {
		return ""
	}
	return rec.Value
}

// Set upserts the DB row, then publishes the new value to the
// notifier. The env-var floor is preserved: a subsequent
// Get will still return the env value if one is set.
//
// The local replica's VersionVector is bumped by one before
// the row is written; concurrent remote writes will be
// reconciled on Merge.
func (r *Resolver) Set(ctx context.Context, key, value, updatedBy string) error {
	cur, err := r.store.GetSystemConfig(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		cur = CRDTRecord{Key: key}
	} else if err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	cur.Value = value
	cur.UpdatedBy = updatedBy
	cur.WriteTS = NextWriteTS()
	cur.VersionVector = IncVector(cur.VersionVector, r.replicaID)
	cur.Tombstone = false
	cur.ReplicaID = r.replicaID
	if err := r.store.SetSystemConfig(ctx, cur); err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	if r.notif != nil {
		if err := r.notif.Publish(key, value); err != nil {
			return fmt.Errorf("settings: notifier %q: %w", key, err)
		}
	}
	return nil
}

// Delete writes a single tombstone upsert via SetSystemConfig
// and publishes the empty value. After Delete, Get returns
// the env value (or "" if env is unset). Delete is atomic:
// one SQL upsert, no DELETE+INSERT pair that would race a
// concurrent Set. Delete is idempotent: calling it twice
// leaves a single tombstoned row, not an error.
//
// ponytail: a DELETE-then-INSERT pair could lose a
// concurrent Set that lands between the two statements;
// the single-upsert design keeps the tombstone and any
// in-flight Set's vector against the same row.
func (r *Resolver) Delete(ctx context.Context, key string) error {
	cur, err := r.store.GetSystemConfig(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		cur = CRDTRecord{Key: key}
	} else if err != nil {
		return fmt.Errorf("settings: delete %q: %w", key, err)
	}
	cur.Value = ""
	cur.UpdatedBy = ""
	cur.WriteTS = NextWriteTS()
	cur.VersionVector = IncVector(cur.VersionVector, r.replicaID)
	cur.Tombstone = true
	cur.ReplicaID = r.replicaID
	if err := r.store.SetSystemConfig(ctx, cur); err != nil {
		return fmt.Errorf("settings: tombstone %q: %w", key, err)
	}
	if r.notif != nil {
		if err := r.notif.Publish(key, ""); err != nil {
			return fmt.Errorf("settings: notifier %q: %w", key, err)
		}
	}
	return nil
}

// List returns every non-tombstoned row. Used by the API GET
// endpoint. Tombstones are filtered out so callers see only
// live values.
func (r *Resolver) List(ctx context.Context) ([]CRDTRecord, error) {
	records, err := r.store.ListSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CRDTRecord, 0, len(records))
	for _, rec := range records {
		if rec.Tombstone {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

// HasDefault reports whether a hardcoded default exists for the
// key. Always false here; callers layer their own defaults
// in the appropriate package (OTel, LLM registry, ...).
func (r *Resolver) HasDefault(_ string) bool { return false }

// Merge folds remote records into the local store. Used by
// replication tests; the production daemon does not call this
// directly because there is no replication peer. Per-key the
// merge is the LWW semantics in Merge; the store applies the
// resulting record with a single SetSystemConfig call.
func (r *Resolver) Merge(ctx context.Context, replicaID string, records []CRDTRecord) error {
	if r.replicaID == "" {
		return errors.New("settings: merge: resolver has no replica id")
	}
	return r.store.MergeSystemConfig(ctx, replicaID, records)
}
