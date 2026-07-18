// Package kmsbyok is a real KMS-backed KeyProvider for production
// deployments. The Provider wraps a KMSClient (typically the AWS
// KMS adapter in aws.go) and caches the plaintext data key. AWS
// is the only supported KMS in this build; swapping providers is
// a one-line change at the construction site (just supply a
// different KMSClient).
package kmsbyok

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrKMSClientRequired is returned when the Provider is
// constructed without a KMSClient. Production tenants MUST wire a
// real AWS KMS client; the only consumers allowed to use the
// nil-client path are tests that explicitly inject a test double
// via Config.AllowTestDouble.
var ErrKMSClientRequired = errors.New("kmsbyok: KMSClient required (production); tests must set AllowTestDouble")

// Config is the per-deployment configuration. KeyID is the KMS
// key ARN; Region is the AWS region for the SDK config; KMSClient
// is the consumer-supplied adapter (the AWS SDK adapter is in
// aws.go). AllowTestDouble is true only in tests; production
// constructors reject an unset KMSClient.
type Config struct {
	KeyID           string
	Region          string
	KMSClient       KMSClient
	AllowTestDouble bool
}

// KMSClient is the consumer-defined interface that abstracts the
// AWS SDK. Production wiring supplies the AWS SDK adapter; tests
// supply a deterministic fake.
type KMSClient interface {
	GenerateDataKey(ctx context.Context, keyID string) (plaintext []byte, err error)
}

// Provider is the in-process KMS-backed KeyProvider. It is
// concurrency-safe; the underlying KMSClient is responsible for
// thread-safety.
type Provider struct {
	mu    sync.Mutex
	cfg   Config
	cache []byte
}

// New returns a Provider that talks to the configured KMSClient.
// Production paths MUST supply a non-nil KMSClient; tests that
// need the deterministic test double must set AllowTestDouble.
func New(cfg Config) (*Provider, error) {
	if cfg.KeyID == "" {
		return nil, errors.New("kmsbyok: KeyID required")
	}
	if cfg.KMSClient == nil && !cfg.AllowTestDouble {
		return nil, ErrKMSClientRequired
	}
	return &Provider{cfg: cfg}, nil
}

// Key returns the cached plaintext key, fetching it from KMS on
// the first call. Subsequent calls return the cached value
// without re-contacting KMS; Rotate invalidates the cache so the
// next call re-fetches.
func (p *Provider) Key(ctx context.Context) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cache) == 32 {
		out := make([]byte, 32)
		copy(out, p.cache)
		return out, nil
	}
	if p.cfg.KMSClient == nil {
		out := deterministicTestKey(p.cfg.KeyID)
		p.cache = out
		return append([]byte(nil), out...), nil
	}
	pt, err := p.cfg.KMSClient.GenerateDataKey(ctx, p.cfg.KeyID)
	if err != nil {
		return nil, fmt.Errorf("kmsbyok: generate data key: %w", err)
	}
	if len(pt) != 32 {
		return nil, fmt.Errorf("kmsbyok: expected 32-byte plaintext, got %d", len(pt))
	}
	p.cache = pt
	return append([]byte(nil), pt...), nil
}

// Rotate invalidates the cached key so the next Key() call re-
// fetches from KMS. Returns nil on success; the underlying KMS
// call's error is propagated when the next Key() runs.
func (p *Provider) Rotate(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = nil
	return nil
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
