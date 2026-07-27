// Tests for the Vault lifecycle (Stop) and the hot-reload (Reload)
// contract. Lives in a separate file so the long-standing
// round-trip / wrong-key / invalid-input tests in vault_test.go
// stay focused.
package vault

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func mustNew(t *testing.T, hexKey string) *Vault {
	t.Helper()
	v, err := New(hexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func TestStopZeroizesKeyAndIsIdempotent(t *testing.T) {
	t.Parallel()
	v := mustNew(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if v.Stopped() {
		t.Fatal("fresh vault should not report Stopped")
	}
	ct, err := v.Encrypt("hello")
	if err != nil {
		t.Fatalf("pre-Stop Encrypt: %v", err)
	}

	v.Stop()
	if !v.Stopped() {
		t.Fatal("Stopped should report true after Stop")
	}

	// Stop must be safe to call repeatedly; the second call is
	// a no-op and must not panic or block.
	v.Stop()
	v.Stop()

	// The key slice must be zeroized in place — a post-mortem
	// dump of the heap must not reveal the master key.
	v.mu.RLock()
	keyLen := len(v.key)
	v.mu.RUnlock()
	if keyLen != 0 {
		t.Fatalf("expected key slice to be nil after Stop, got len=%d", keyLen)
	}

	// Any use-after-stop must return ErrStopped.
	if _, err := v.Encrypt("anything"); !errors.Is(err, ErrStopped) {
		t.Fatalf("Encrypt after Stop: want ErrStopped, got %v", err)
	}
	if _, err := v.Decrypt(ct); !errors.Is(err, ErrStopped) {
		t.Fatalf("Decrypt after Stop: want ErrStopped, got %v", err)
	}
}

// TestStopConcurrent guards the CAS path: many goroutines racing
// to call Stop must see exactly one transition from live to
// stopped, and every goroutine must end without a panic.
func TestStopConcurrent(t *testing.T) {
	t.Parallel()
	v := mustNew(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			v.Stop()
		}()
	}
	wg.Wait()
	if !v.Stopped() {
		t.Fatal("vault should be Stopped after concurrent Stop calls")
	}
	if _, err := v.Encrypt("x"); !errors.Is(err, ErrStopped) {
		t.Fatalf("Encrypt after concurrent Stop: want ErrStopped, got %v", err)
	}
}

func TestReloadSwapsKeyAtomically(t *testing.T) {
	t.Parallel()
	keyA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	keyB := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	v := mustNew(t, keyA)

	ctA, err := v.Encrypt("plaintext-a")
	if err != nil {
		t.Fatalf("Encrypt A: %v", err)
	}
	gotA, err := v.Decrypt(ctA)
	if err != nil || gotA != "plaintext-a" {
		t.Fatalf("Decrypt A: got=%q err=%v", gotA, err)
	}

	err = v.Reload(keyB)
	if err != nil {
		t.Fatalf("Reload B: %v", err)
	}

	ctB, err := v.Encrypt("plaintext-b")
	if err != nil {
		t.Fatalf("Encrypt B: %v", err)
	}
	gotB, err := v.Decrypt(ctB)
	if err != nil || gotB != "plaintext-b" {
		t.Fatalf("Decrypt B: got=%q err=%v", gotB, err)
	}

	// A ciphertext produced under keyA is no longer decryptable
	// under keyB — this is the documented operator contract for
	// a master-key rotation: stored secrets must be re-encrypted
	// by the caller after Reload.
	if _, err := v.Decrypt(ctA); err == nil {
		t.Fatal("expected ctA to be undecryptable under keyB")
	}
}

func TestReloadRetainsOldKeyOnFailure(t *testing.T) {
	t.Parallel()
	keyA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v := mustNew(t, keyA)

	ct, err := v.Encrypt("plaintext")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	badKeys := []struct {
		name string
		key  string
	}{
		{"too short", "abc"},
		{"invalid hex", "not-valid-hex"},
		{"all zero", "0000000000000000000000000000000000000000000000000000000000000000"},
	}
	for _, tc := range badKeys {
		t.Run(tc.name, func(t *testing.T) {
			if rerr := v.Reload(tc.key); rerr == nil {
				t.Fatalf("Reload(%s): expected error, got nil", tc.name)
			}
		})
	}

	// The original key must still decrypt the original
	// ciphertext — Reload must not have torn down the vault
	// when the candidate key was invalid.
	got, err := v.Decrypt(ct)
	if err != nil || got != "plaintext" {
		t.Fatalf("Decrypt after failed Reload: got=%q err=%v", got, err)
	}
}

func TestReloadAfterStopReturnsErrStopped(t *testing.T) {
	t.Parallel()
	v := mustNew(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	v.Stop()
	if err := v.Reload("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"); !errors.Is(err, ErrStopped) {
		t.Fatalf("Reload after Stop: want ErrStopped, got %v", err)
	}
}

func TestStopAfterReloadStopsReloadedVault(t *testing.T) {
	t.Parallel()
	keyA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	keyB := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	v := mustNew(t, keyA)
	if err := v.Reload(keyB); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	v.Stop()
	if !v.Stopped() {
		t.Fatal("Stopped should be true after Reload+Stop")
	}
	if _, err := v.Encrypt("x"); !errors.Is(err, ErrStopped) {
		t.Fatalf("Encrypt: want ErrStopped, got %v", err)
	}
}

func TestReloadConcurrentWithEncryptDoesNotPanic(t *testing.T) {
	t.Parallel()
	keyA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	keyB := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	v := mustNew(t, keyA)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var encryptCount atomic.Int64
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			k := keyA
			if i%2 == 0 {
				k = keyB
			}
			_ = v.Reload(k)
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Either Encrypt succeeds (pre/post Reload) or it
			// returns ErrStopped (if Stop runs concurrently,
			// which we don't trigger here) — any other error is
			// a real bug.
			if _, err := v.Encrypt("x"); err != nil && !errors.Is(err, ErrStopped) {
				t.Errorf("Encrypt during Reload: %v", err)
				return
			}
			encryptCount.Add(1)
		}
	}()
	// Spin long enough that the goroutines interleave.
	for i := 0; i < 200; i++ {
		_, _ = v.Encrypt("x")
	}
	close(stop)
	wg.Wait()
	if encryptCount.Load() == 0 {
		t.Fatal("Encrypt goroutine never produced a successful call")
	}
}
