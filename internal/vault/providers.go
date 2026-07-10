// Package vault owns encryption-at-rest for sensitive data
// (LLM API keys, plugin credentials) plus the master-key
// provider abstraction needed for BYOK and KMS integration.
//
// Per ADR-0004 the data-encryption algorithm is fixed at AES-256-GCM
// with a 32-byte master key. The MASTER KEY SOURCE, however, was
// previously pinned to the PROMPTSHEON_VAULT_KEY environment
// variable. That pinning prevented production tenants from
// plugging in a managed-key service (AWS KMS, HashiCorp Vault,
// etc.) and from rotating the master key without restarting
// every daemon instance.
//
// Tier 2.45 of the architecture review board introduces the
// KeyProvider interface that abstracts the master-key source.
// The local AES-256-GCM Vault implementation is unchanged;
// only the way it gets its key has been generalised.
//
// SecretBroker is the consumer-facing capability for callers
// who hold short-lived, single-purpose credentials (openai api
// key, anthropic api key, plugin secret reference). Today the
// openai implementations in internal/llm read the key from a
// ProviderConfig; Tier 2.45 introduces a SecretBroker path so
// keys never live in long-lived ProviderConfig copies.
package vault

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
)

// hexDecode is a thin wrapper around hex.DecodeString so the
// helper body is grouped with the rest of the file (rather than
// scattered). It exists primarily for tests that want to stub it.
func hexDecode(s string) ([]byte, error) { return hex.DecodeString(s) }

// hexEncode is a thin wrapper around hex.EncodeToString for the
// same reason as hexDecode.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// KeyProvider is the consumer-defined contract for sourcing the
// master key. Production wiring supplies an EnvKeyProvider, a
// KMSKeyProvider (M3 follow-on), or a file-based provider for
// air-gapped deployments.
type KeyProvider interface {
	// Key returns the 32-byte master key. Implementations may
	// return a non-nil error when the key is unavailable; the
	// Vault constructor translates that error into a startup
	// failure (per Charter Principle 5, the daemon fails closed).
	Key(ctx context.Context) ([]byte, error)
	// Rotate returns the next master key. The Vault constructor
	// calls Rotate during the key-rotation handshake (an
	// admin-triggered command, not a hot path). A nil error from
	// the caller is a no-op.
	Rotate(ctx context.Context) error
}

// SecretBroker is the consumer-defined contract for issuing
// short-lived credentials to plugins and provider calls. Today it
// is a thin pass-through over the master-key Vault; production
// wiring supplies a remote-Secrets-Manager-backed implementation.
type SecretBroker interface {
	Resolve(ctx context.Context, secretID string) ([]byte, error)
}

// ErrUnknownSecret is returned by SecretBroker implementations
// when the requested secret identifier is not present in the
// underlying store.
var ErrUnknownSecret = errors.New("vault: unknown secret")

// ErrKeyUnavailable is returned by KeyProvider implementations
// when no master key is currently available.
var ErrKeyUnavailable = errors.New("vault: master key unavailable")

// EnvKeyProvider sources the master key from an environment
// variable. It is the production-default today; KMSKeyProvider
// replaces it for tenants that have a managed-key service.
type EnvKeyProvider struct {
	VarName string
}

// NewEnvKeyProvider returns an EnvKeyProvider that reads the
// master key from PROMPTSHEON_VAULT_KEY.
func NewEnvKeyProvider() *EnvKeyProvider {
	return &EnvKeyProvider{VarName: "PROMPTSHEON_VAULT_KEY"}
}

// LoadFromEnv returns the EnvKeyProvider's master key by reading
// the OS environment. Errors include the missing-key case for the
// installer script and the bad-format case for malformed keys.
func LoadFromEnv(varName string) ([]byte, error) {
	v := os.Getenv(varName)
	if v == "" {
		return nil, ErrKeyUnavailable
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, errors.New("vault: env master key length wrong")
	}
	return b, nil
}

// Key implements KeyProvider.
func (p *EnvKeyProvider) Key(ctx context.Context) ([]byte, error) {
	return LoadFromEnv(p.VarName)
}

// Rotate is a no-op for EnvKeyProvider; env var rotation requires
// an out-of-band restart of the daemon.
func (p *EnvKeyProvider) Rotate(context.Context) error {
	return errors.New("vault: env provider requires restart to rotate")
}

// StaticKeyProvider returns a fixed key. Used by tests and by
// air-gapped deployments where the key is loaded from a sealed
// config file at boot.
type StaticKeyProvider struct {
	Key32 []byte
}

// NewStaticKeyProvider returns a StaticKeyProvider holding the
// supplied 32-byte key.
func NewStaticKeyProvider(key []byte) *StaticKeyProvider {
	return &StaticKeyProvider{Key32: key}
}

// Key implements KeyProvider.
func (p *StaticKeyProvider) Key(context.Context) ([]byte, error) {
	out := make([]byte, len(p.Key32))
	copy(out, p.Key32)
	return out, nil
}

// Rotate is a no-op for StaticKeyProvider.
func (p *StaticKeyProvider) Rotate(context.Context) error { return nil }

// StaticSecretBroker is the SecretBroker produced by the bundled
// Vault. It accepts a list of {id -> ciphertext} pairs and resolves
// them by Vault.Decrypt.
type StaticSecretBroker struct {
	Vault   *Vault
	Secrets map[string][]byte
}

// NewStaticSecretBroker constructs a broker that resolves
// pre-encrypted secrets at startup time.
func NewStaticSecretBroker(v *Vault, secrets map[string][]byte) *StaticSecretBroker {
	return &StaticSecretBroker{Vault: v, Secrets: secrets}
}

// Resolve implements SecretBroker.
func (b *StaticSecretBroker) Resolve(_ context.Context, secretID string) ([]byte, error) {
	ct, ok := b.Secrets[secretID]
	if !ok {
		return nil, ErrUnknownSecret
	}
	pt, err := b.Vault.Decrypt(string(ct))
	if err != nil {
		return nil, err
	}
	return []byte(pt), nil
}

// BuildKeyFromEnv composes the legacy PROMPTSHEON_VAULT_KEY env var
// path into a Vault, preserving exact backwards compatibility
// while allowing production tenants to swap the source by
// implementing KeyProvider and passing it to BuildVault. The
// function never returns nil for a non-empty hex key; the
// all-zero key path in *Vault rejects a trivially-decryptable
// key.
func BuildVault(p KeyProvider) (*Vault, error) {
	key, err := p.Key(context.Background())
	if err != nil {
		return nil, err
	}
	v, err := New(hexEncode(key))
	if err != nil {
		return nil, err
	}
	return v, nil
}

// BuildEnvVault is the backwards-compat constructor: it reads the
// env var, validates the key, and returns a Vault. New code should
// use BuildVault with an explicit KeyProvider.
func BuildEnvVault() (*Vault, error) {
	return BuildVault(NewEnvKeyProvider())
}
