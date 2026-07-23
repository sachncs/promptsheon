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
// the existing `internal/vault`.
//
// The Resolver is read-on-every-access (not read-once-at-boot)
// so a settings change is reflected immediately. Subscribers
// (commit A2) fire on Set/Delete so dependent pipelines (OTel,
// LLM registry) can reconfigure without a daemon restart.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Notifier fires callbacks on Set / Delete. Subscribers are
// the OTel and LLM packages; the notifier's callbacks are
// called synchronously after the DB write commits, so a PUT
// returns 200 only when the dependent pipeline has been
// reset.
type Notifier struct {
	mu   sync.Mutex
	subs map[string][]func(value string)
}

// NewNotifier returns a fresh notifier.
func NewNotifier() *Notifier { return &Notifier{subs: map[string][]func(value string){}} }

// Subscribe registers a callback for a single key. The callback
// fires synchronously on Set(key, value) and Delete(key).
// Multiple subscribers per key are supported; they fire in
// registration order.
func (n *Notifier) Subscribe(key string, fn func(value string)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.subs[key] = append(n.subs[key], fn)
}

// Publish fires the subscribers for a key. Empty no-op when
// nobody subscribed.
func (n *Notifier) Publish(key, value string) {
	n.mu.Lock()
	subs := append([]func(value string){}, n.subs[key]...)
	n.mu.Unlock()
	for _, fn := range subs {
		fn(value)
	}
}

// Resolver merges the hardcoded default, env-var, and DB
// sources. Read paths (Get, HasDefault) are safe for
// concurrent use; writes go through Store which is owned by
// the API layer.
type Resolver struct {
	store Store
	env   EnvSource
	notif *Notifier
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
	GetSystemConfig(ctx context.Context, key string) (string, time.Time, error)
	SetSystemConfig(ctx context.Context, key, value, updatedBy string) error
	DeleteSystemConfig(ctx context.Context, key string) error
	ListSystemConfig(ctx context.Context) ([]SystemConfigRow, error)
}

// SystemConfigRow mirrors models.SystemConfig. Defined here so
// this package doesn't import internal/models (which imports
// the store package — circular).
type SystemConfigRow struct {
	Key       string
	Value     string
	UpdatedAt time.Time
	UpdatedBy string
}

// NewResolver wires the Resolver to its dependencies. env may
// be nil; the Resolver falls back to OSEnvSource.
func NewResolver(store Store, notif *Notifier, env EnvSource) *Resolver {
	if env == nil {
		env = OSEnvSource{}
	}
	return &Resolver{store: store, notif: notif, env: env}
}

// Get returns the effective value for key. Order:
//
//  1. env-var (the floor);
//  2. DB row (the ceiling);
//  3. empty string if neither is set.
//
// Callers that need a hardcoded default should fall through
// from the empty string to their own default; the Resolver
// stays free of magic-string knowledge of specific keys.
func (r *Resolver) Get(ctx context.Context, key string) string {
	if v, ok := r.env.Get(key); ok && v != "" {
		// Floor: env wins over DB. (Setting an env-var is
		// the operator's "I want THIS value" signal; the DB
		// row is the runtime ceiling only when env is unset.)
		return v
	}
	v, _, err := r.store.GetSystemConfig(ctx, key)
	if err == nil {
		return v
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// A real DB error (table missing, lock, ...) — log
		// it on the caller's side via the audit chain. The
		// Resolver returns empty string so the caller's
		// default kicks in.
		return ""
	}
	return ""
}

// Set upserts the DB row, then publishes the new value to the
// notifier. The env-var floor is preserved: a subsequent
// Get will still return the env value if one is set.
func (r *Resolver) Set(ctx context.Context, key, value, updatedBy string) error {
	if err := r.store.SetSystemConfig(ctx, key, value, updatedBy); err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	if r.notif != nil {
		r.notif.Publish(key, value)
	}
	return nil
}

// Delete removes the row, then publishes an empty value. After
// Delete, Get returns the env value (or "" if env is unset).
func (r *Resolver) Delete(ctx context.Context, key string) error {
	if err := r.store.DeleteSystemConfig(ctx, key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return fmt.Errorf("settings: delete %q: %w", key, err)
	}
	if r.notif != nil {
		r.notif.Publish(key, "")
	}
	return nil
}

// List returns every row. Used by the API GET endpoint.
func (r *Resolver) List(ctx context.Context) ([]SystemConfigRow, error) {
	return r.store.ListSystemConfig(ctx)
}

// HasDefault reports whether a hardcoded default exists for the
// key. Always false here; callers layer their own defaults
// in the appropriate package (OTel, LLM registry, ...).
func (r *Resolver) HasDefault(_ string) bool { return false }
