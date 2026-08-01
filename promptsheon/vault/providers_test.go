//go:build tests_migration


package vault

import (
	"context"
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

func TestEnvKeyProviderMissing(t *testing.T) {
	t.Parallel()
	got, err := LoadFromEnv("PROMPTSHEON_VAULT_KEY_NOT_SET_XYZ")
	if !errors.Is(err, errs.ErrVaultKeyUnavail) {
		t.Fatalf("expected errs.ErrVaultKeyUnavail, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil key, got %v", got)
	}
}

func TestEnvKeyProviderBadHex(t *testing.T) {
	t.Setenv("PROMPTSHEON_VAULT_KEY_BADHEX", "not-hex")
	got, err := LoadFromEnv("PROMPTSHEON_VAULT_KEY_BADHEX")
	if err == nil {
		t.Fatalf("expected error, got key %v", got)
	}
}

func TestEnvKeyProviderWrongLength(t *testing.T) {
	t.Setenv("PROMPTSHEON_VAULT_KEY_SHORT", "deadbeef")
	got, err := LoadFromEnv("PROMPTSHEON_VAULT_KEY_SHORT")
	if err == nil {
		t.Fatalf("expected length error, got key %v", got)
	}
}

func TestStaticKeyProviderReturnsKeyCopy(t *testing.T) {
	t.Parallel()
	k := make([]byte, 32)
	for i := range k {
		k[i] = 0x42
	}
	p := NewStaticKeyProvider(k)
	out, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	if len(out) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(out))
	}
	// Mutating the source after the call must not change the returned slice.
	k[0] = 0x99
	if out[0] == 0x99 {
		t.Fatalf("Key did not return a copy")
	}
}

func TestStaticKeyProviderRotateIsNoop(t *testing.T) {
	t.Parallel()
	k := make([]byte, 32)
	p := NewStaticKeyProvider(k)
	if err := p.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
}

func TestStaticSecretBrokerUnknownSecret(t *testing.T) {
	t.Parallel()
	v, err := BuildVault(NewEnvKeyProvider())
	if err == nil {
		// Skip if env not set; the no-env case is exercised above.
		v = &Vault{}
	}
	b := NewStaticSecretBroker(v, map[string][]byte{})
	if _, err := b.Resolve(context.Background(), "missing"); !errors.Is(err, errs.ErrVaultUnknown) {
		t.Fatalf("expected errs.ErrVaultUnknown, got %v", err)
	}
}
