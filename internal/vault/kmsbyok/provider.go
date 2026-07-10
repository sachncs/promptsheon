// Package kmsbyok is a stub KMS-backed KeyProvider for production
// deployments. The shape matches the architecture review board's
// Tier 2.45 follow-on plan: a thin wrapper around the AWS KMS
// GenerateDataKey API. The full SDK dependency ships in M3
// follow-on; today's commit delivers the value type, the
// production-time configuration, and the deterministic test
// double that lets the daemon boot in CI without an AWS account.
package kmsbyok

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Config is the per-deployment configuration. Today it is read
// from PROMPTSHEON_KMS_KEY_ID plus the standard AWS SDK env
// (region, credentials); the real path constructs an AWS KMS
// client and calls GenerateDataKey. The stub returns a
// deterministic test key when KMSClient is nil.
type Config struct {
	KeyID     string
	Region    string
	KMSClient KMSClient
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
	mu   sync.Mutex
	cfg  Config
	cache []byte
}

// New returns a Provider that talks to the configured KMSClient
// (or a deterministic test double when cfg.KMSClient is nil).
func New(cfg Config) (*Provider, error) {
	if cfg.KeyID == "" {
		return nil, errors.New("kmsbyok: KeyID required")
	}
	return &Provider{cfg: cfg}, nil
}

// Key returns the cached plaintext key, fetching it from KMS on
// the first call. The plaintext key is cached for the lifetime
// of the daemon; rotation is a M3 follow-on (Rotate is a no-op
// today because the AWS SDK adapter is not in this build).
func (p *Provider) Key(ctx context.Context) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cache) == 32 {
		out := make([]byte, 32)
		copy(out, p.cache)
		return out, nil
	}
	if p.cfg.KMSClient == nil {
		// Test double: deterministic 32-byte key derived from KeyID.
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

// Rotate is a no-op today; the AWS SDK adapter is M3 follow-on.
func (p *Provider) Rotate(context.Context) error {
	return nil
}

// deterministicTestKey returns a stable 32-byte key derived from
// the supplied KMS keyID. Used when no KMSClient is configured.
// The derivation is intentionally simple: SHA-256 of the keyID
// (after the literal test-prefix marker).
func deterministicTestKey(keyID string) []byte {
	const marker = "kmsbyok-test-v1:"
	return sha256Sum([]byte(marker + keyID))
}

func sha256Sum(b []byte) []byte {
	// Lightweight inline implementation to avoid importing
	// crypto/sha256 in this small file. The function is consumed
	// only by the test double.
	return inlineSHA256(b)
}

func inlineSHA256(b []byte) []byte {
	return sha256Hash(b)
}
