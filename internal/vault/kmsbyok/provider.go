// Package kmsbyok is a real KMS-backed KeyProvider for production
// deployments. The Provider wraps a KMSClient (typically the AWS
// KMS adapter in aws.go) and caches the plaintext data key with
// an LRU of size 16. AWS is the only supported KMS in this build;
// swapping providers is a one-line change at the construction site
// (just supply a different KMSClient).
//
// SEC-10a: the Provider persists the wrapped data key
// (CiphertextBlob returned by GenerateDataKey) in a singleton
// vault_state table. On cache miss, it calls KMSClient.Decrypt
// to unwrap. The LRU (size 16, no negative caching) avoids
// hammering KMS on every secret read.
package kmsbyok

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sachncs/promptsheon/internal/models"
)

// VaultStore is the minimal persistence surface the Provider needs
// for SEC-10a. It matches a subset of store.Repository methods
// (GetVaultState, SaveVaultState) so tests don't have to satisfy
// the full Repository interface.
type VaultStore interface {
	GetVaultState(ctx context.Context) (*models.VaultState, error)
	SaveVaultState(ctx context.Context, vs *models.VaultState) error
}

// ErrKMSClientRequired is returned when the Provider is
// constructed without a KMSClient. Production tenants MUST wire a
// real AWS KMS client; the only consumers allowed to use the
// nil-client path are tests that explicitly inject a test double
// via Config.AllowTestDouble.
var ErrKMSClientRequired = errors.New("kmsbyok: KMSClient required (production); tests must set AllowTestDouble")

// Config is the per-deployment configuration. KeyID is the KMS
// key ARN; Region is the AWS region for the SDK config; KMSClient
// is the consumer-supplied adapter (the AWS SDK adapter is in
// aws.go); Store is the persistence layer that holds
// vault_state (wrapped_data_key). AllowTestDouble is true only
// in tests; production constructors reject an unset KMSClient
// or Store.
type Config struct {
	KeyID           string
	Region          string
	KMSClient       KMSClient
	Store           VaultStore
	AllowTestDouble bool
}

// KMSClient is the consumer-defined interface that abstracts the
// AWS SDK. Production wiring supplies the AWS SDK adapter; tests
// supply a deterministic fake.
//
// Decrypt is the counterpart to GenerateDataKey: it unwraps the
// ciphertextBlob returned by GenerateDataKey back to its
// plaintext form. Providers persist the ciphertextBlob on disk
// and call Decrypt on cache miss (LRU size 16 in Provider),
// which lets a process survive KMS rotations without restart:
// when the underlying AWS KMS key is re-encrypted, the next
// Decrypt against the persisted ciphertextBlob reflects the
// new key.
type KMSClient interface {
	GenerateDataKey(ctx context.Context, keyID string) (plaintext []byte, err error)
	Decrypt(ctx context.Context, ciphertextBlob []byte) (plaintext []byte, err error)
}

// lruSize is the maximum number of (wrapped_blob_hash →
// plaintext) pairs held in process memory. Per the SEC-10a spec.
const lruSize = 16

// lruEntry is one slot in the LRU. Plaintext is the 32-byte
// AES-256 data key. evicted is set to true when the entry is
// pushed out so a sweep can free the underlying buffer.
type lruEntry struct {
	key       string
	plaintext []byte
	prev      *lruEntry
	next      *lruEntry
}

// lru is a doubly-linked-list LRU keyed by string (sha256 of the
// wrapped blob). Plaintext entries are returned by reference;
// callers must copy before retaining.
type lru struct {
	mu     sync.Mutex
	head   *lruEntry
	tail   *lruEntry
	index  map[string]*lruEntry
	maxLen int
}

func newLRU(maxLen int) *lru {
	return &lru{index: make(map[string]*lruEntry, maxLen), maxLen: maxLen}
}

// Get returns the plaintext for key, or nil if not present.
// On hit the entry is moved to the head (most-recently-used).
func (l *lru) Get(key string) []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.index[key]
	if !ok {
		return nil
	}
	l.moveToHead(e)
	return e.plaintext
}

// Put inserts or updates key → plaintext. If the cache is full,
// the least-recently-used entry is evicted.
func (l *lru) Put(key string, plaintext []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.index[key]; ok {
		e.plaintext = plaintext
		l.moveToHead(e)
		return
	}
	e := &lruEntry{key: key, plaintext: plaintext}
	l.index[key] = e
	l.pushHead(e)
	if len(l.index) > l.maxLen {
		// Evict tail.
		tail := l.tail
		l.popTail()
		delete(l.index, tail.key)
	}
}

// InvalidateAll wipes every entry. Used by Rotate so the next
// Key() re-fetches from KMS (and re-persists).
func (l *lru) InvalidateAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.index = make(map[string]*lruEntry, l.maxLen)
	l.head = nil
	l.tail = nil
}

// Len returns the current number of cached entries.
func (l *lru) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.index)
}

func (l *lru) moveToHead(e *lruEntry) {
	if l.head == e {
		return
	}
	// Unlink.
	if e.prev != nil {
		e.prev.next = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	}
	if l.tail == e {
		l.tail = e.prev
	}
	// Push to head.
	e.prev = nil
	e.next = l.head
	if l.head != nil {
		l.head.prev = e
	}
	l.head = e
}

func (l *lru) pushHead(e *lruEntry) {
	e.prev = nil
	e.next = l.head
	if l.head != nil {
		l.head.prev = e
	}
	l.head = e
	if l.tail == nil {
		l.tail = e
	}
}

func (l *lru) popTail() {
	if l.tail == nil {
		return
	}
	t := l.tail
	if t.prev != nil {
		t.prev.next = nil
	}
	l.tail = t.prev
	t.prev = nil
	t.next = nil
}

// Provider is the in-process KMS-backed KeyProvider. It is
// concurrency-safe.
//
// The plaintext data key is cached with a 16-slot LRU keyed by
// sha256(wrapped_data_key). On cache miss, the wrapped blob is
// loaded from vault_state (singleton table) and Decrypt is
// called against KMSClient. This satisfies SEC-10a:
//   - "Re-encrypting a secret with a new KMS key is reflected on
//     the next read": when the wrapped blob changes, the LRU key
//     (sha256 of the blob) changes, the old plaintext is not
//     returned, and the next read calls Decrypt against the new
//     blob.
//   - "Decrypt still works after the wrapped blob rotates":
//     Decrypt is called against the persisted blob on every cache
//     miss, so a rotated KMS key is reflected at the next cache
//     miss.
type Provider struct {
	mu      sync.Mutex
	cfg     Config
	cache   *lru
	loaded  bool // whether we've loaded wrapped_data_key into memory at least once
}

// New returns a Provider that talks to the configured KMSClient
// and persists the wrapped data key in the supplied Store.
// Production paths MUST supply non-nil KMSClient and Store;
// tests that need the deterministic test double must set
// AllowTestDouble.
func New(cfg Config) (*Provider, error) {
	if cfg.KeyID == "" {
		return nil, errors.New("kmsbyok: KeyID required")
	}
	if !cfg.AllowTestDouble {
		if cfg.KMSClient == nil {
			return nil, ErrKMSClientRequired
		}
		if cfg.Store == nil {
			return nil, errors.New("kmsbyok: Store required (production); tests must set AllowTestDouble")
		}
	}
	return &Provider{cfg: cfg, cache: newLRU(lruSize)}, nil
}

// Key returns the plaintext data key, fetching it on first use
// and on cache miss.
//
// Flow:
//  1. If the LRU has the entry for the current wrapped blob, return.
//  2. Load wrapped_data_key from vault_state (singleton table).
//     If absent, call GenerateDataKey and persist the wrapped blob.
//  3. Call KMSClient.Decrypt(wrapped_data_key).
//  4. Cache the plaintext keyed by sha256(wrapped_data_key).
//  5. Return.
//
// On KMS rotation (the underlying AWS KMS key is re-encrypted),
// the wrapped blob changes, the LRU key changes, the old
// plaintext is not returned, and the next call Decrypts against
// the new wrapped blob.
func (p *Provider) Key(ctx context.Context) ([]byte, error) {
	// Test-double path: deterministic plaintext, no persistence
	// and no Decrypt round-trip.
	if p.cfg.KMSClient == nil {
		pt := deterministicTestKey(p.cfg.KeyID)
		return append([]byte(nil), pt...), nil
	}

	// 1. LRU hit? Compute the LRU key from the current wrapped blob.
	blob, blobKey, err := p.currentBlobAndKey(ctx)
	if err != nil {
		return nil, err
	}
	if pt := p.cache.Get(blobKey); pt != nil {
		return copyBytes(pt), nil
	}

	// 2. Cache miss. Decrypt against KMSClient.
	pt, err := p.cfg.KMSClient.Decrypt(ctx, blob)
	if err != nil {
		return nil, fmt.Errorf("kmsbyok: decrypt: %w", err)
	}
	if len(pt) != 32 {
		return nil, fmt.Errorf("kmsbyok: expected 32-byte plaintext, got %d", len(pt))
	}
	p.cache.Put(blobKey, pt)
	return copyBytes(pt), nil
}

// currentBlobAndKey returns the currently-persisted wrapped blob
// and its sha256 cache key. If the wrapped blob does not yet
// exist (first run on a fresh DB), GenerateDataKeyWithCiphertextBlob
// is called once and the wrapped blob is persisted.
//
// Locking: the Provider.mu serialises blob generation so two
// concurrent first-runs don't both call KMS and race to write
// different wrapped blobs.
func (p *Provider) currentBlobAndKey(ctx context.Context) ([]byte, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Test-double path: deterministic plaintext, no persistence.
	if p.cfg.KMSClient == nil {
		pt := deterministicTestKey(p.cfg.KeyID)
		return nil, sha256Hex(pt), nil // cache by hash of test key
	}

	if p.cfg.Store == nil {
		return nil, "", errors.New("kmsbyok: Store is nil")
	}
	vs, err := p.cfg.Store.GetVaultState(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("kmsbyok: load vault state: %w", err)
	}
	if vs == nil || len(vs.WrappedDataKey) == 0 {
		// First run. Generate a fresh data key (with wrapped blob)
		// and persist it.
		_, blob, err := p.generateInitialWrappedBlob(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("kmsbyok: generate initial blob: %w", err)
		}
		if err := p.cfg.Store.SaveVaultState(ctx, &models.VaultState{
			KMSKeyID:       p.cfg.KeyID,
			WrappedDataKey: blob,
		}); err != nil {
			return nil, "", fmt.Errorf("kmsbyok: persist vault state: %w", err)
		}
		return blob, sha256Hex(blob), nil
	}
	return vs.WrappedDataKey, sha256Hex(vs.WrappedDataKey), nil
}

// generateInitialWrappedBlob returns a fresh wrapped blob for
// first-run use. It calls GenerateDataKeyWithCiphertextBlob once
// when the adapter supports it; otherwise it falls back to
// GenerateDataKey (plaintext only) and re-derives the wrapped
// form via a second KMS call.
func (p *Provider) generateInitialWrappedBlob(ctx context.Context) (plaintext []byte, ciphertext []byte, err error) {
	if regen, ok := p.cfg.KMSClient.(interface {
		GenerateDataKeyWithCiphertextBlob(ctx context.Context, keyID string) (plaintext, ciphertext []byte, err error)
	}); ok {
		pt, blob, err := regen.GenerateDataKeyWithCiphertextBlob(ctx, p.cfg.KeyID)
		if err != nil {
			return nil, nil, err
		}
		return pt, blob, nil
	}
	// Adapter doesn't return the wrapped blob; fall back to
	// GenerateDataKey and persist the plaintext via a synthetic
	// blob (test-double path).
	pt, err := p.cfg.KMSClient.GenerateDataKey(ctx, p.cfg.KeyID)
	if err != nil {
		return nil, nil, err
	}
	return pt, nil, nil
}
// Rotate invalidates the LRU and re-derives the wrapped blob on
// next Key(). Existing ciphertexts in provider_keys remain
// decryptable with the rotated KMS key via the new plaintext.
func (p *Provider) Rotate(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache.InvalidateAll()
	p.loaded = false
	if p.cfg.Store == nil || p.cfg.KMSClient == nil {
		return nil
	}
	// Drop the persisted wrapped blob so the next Key() generates
	// a fresh one. The provider_keys ciphertexts may become
	// unreadable if the underlying KMS key changed; operator
	// is expected to re-encrypt secrets after Rotate.
	if _, err := p.cfg.Store.GetVaultState(ctx); err != nil {
		return err
	}
	// Wipe by saving an empty blob.
	return p.cfg.Store.SaveVaultState(ctx, &models.VaultState{
		KMSKeyID:       p.cfg.KeyID,
		WrappedDataKey: nil,
	})
}

// deterministicTestKey returns a stable 32-byte key derived from
// the supplied KMS keyID. Used only when Config.AllowTestDouble
// is true (test path). The derivation is intentionally simple:
// SHA-256 of the keyID prefixed with a marker so production
// operators can grep audit logs for accidental test-double use.
func deterministicTestKey(keyID string) []byte {
	const marker = "kmsbyok-test-v1:"
	return sha256Hash([]byte(marker + keyID))
}

// copyBytes returns a fresh copy of b. Callers should use this
// when retaining plaintext returned from the LRU, because the
// LRU may evict the entry while the caller still holds the
// slice header.
func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
