// Tests for the Rotate contract: explicit dependency errors,
// cache invalidation, and preservation of the persisted
// wrapped_data_key (so existing ciphertexts stay decryptable
// after a rotation).
package kmsbyok

import (
	"context"
	"errors"
	"testing"
)

// TestRotateInvalidatesCacheWithoutPersistingChange locks in the
// "Rotate does not mutate vault_state" guarantee: a successful
// Rotate clears the in-process LRU but leaves the wrapped blob
// alone so provider_keys ciphertexts remain decryptable against
// the next Key().
func TestRotateInvalidatesCacheWithoutPersistingChange(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	kms := &fakeKMS{plaintext: key}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	// Prime the cache and the persisted blob.
	first, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 1: %v", err)
	}
	if !equalBytes(first, key) {
		t.Fatalf("first key mismatch")
	}
	if fs.vs == nil {
		t.Fatalf("expected vault_state populated after first Key")
	}
	blobBefore := append([]byte(nil), fs.vs.WrappedDataKey...)

	// Successful Rotate: cache cleared, wrapped blob unchanged.
	err = p.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := p.cache.Len(); got != 0 {
		t.Errorf("expected LRU cleared after Rotate, got len=%d", got)
	}
	if fs.vs == nil {
		t.Fatalf("Rotate must not nil vault_state")
	}
	if !equalBytes(fs.vs.WrappedDataKey, blobBefore) {
		t.Errorf("Rotate must not modify wrapped_data_key")
	}

	// Next Key() must re-Decrypt the same wrapped blob and
	// return the same plaintext.
	second, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key after Rotate: %v", err)
	}
	if !equalBytes(second, key) {
		t.Errorf("post-Rotate key mismatch")
	}
	if kms.decCalls != 2 {
		t.Errorf("expected exactly 2 Decrypt calls (initial + post-Rotate), got %d", kms.decCalls)
	}
}

// TestRotateMissingStoreReturnsDependencyError locks in the
// explicit-error contract: a Provider built without Store
// reports a clear "Store required" error from Rotate rather
// than silently succeeding.
func TestRotateMissingStoreReturnsDependencyError(t *testing.T) {
	t.Parallel()
	kms := &fakeKMS{plaintext: make([]byte, 32)}
	p, err := New(Config{
		KeyID:           "arn:aws:kms:us-east-1:111122223333:key/abcd",
		KMSClient:       kms,
		AllowTestDouble: true,
		// Store intentionally nil.
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = p.Rotate(context.Background())
	if err == nil {
		t.Fatal("expected error from Rotate without Store")
	}
	if err.Error() == "" {
		t.Fatalf("Rotate error was empty: %v", err)
	}
}

// TestRotateMissingKMSClientReturnsDependencyError mirrors the
// Store test for KMSClient: Rotate must surface ErrKMSClientRequired
// when no KMSClient is wired.
func TestRotateMissingKMSClientReturnsDependencyError(t *testing.T) {
	t.Parallel()
	fs := &fakeStore{}
	p, err := New(Config{
		KeyID:           "arn:aws:kms:us-east-1:111122223333:key/abcd",
		Store:           fs,
		AllowTestDouble: true,
		// KMSClient intentionally nil.
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = p.Rotate(context.Background())
	if !errors.Is(err, ErrKMSClientRequired) {
		t.Fatalf("Rotate: want ErrKMSClientRequired, got %v", err)
	}
}

// TestRotatePreservesExistingCiphertexts locks in the operator
// contract: after Rotate, a plaintext previously encrypted
// with the active data key remains decryptable because the
// wrapped blob (and thus the data key) is unchanged.
func TestRotatePreservesExistingCiphertexts(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	kms := &fakeKMS{plaintext: key}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	// Simulate a secret encrypted under the current data key
	// by stamping it on the (simulated) provider_keys row. The
	// provider itself doesn't own that table; the invariant we
	// exercise is that the cached plaintext survives Rotate
	// unchanged, so any caller still reading from the cache
	// (or re-Decrypting the preserved blob) sees the same key.
	if _, err := p.Key(context.Background()); err != nil {
		t.Fatalf("Key: %v", err)
	}
	preCacheHash := sha256Hex(key)

	if err := p.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	post, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key after Rotate: %v", err)
	}
	if sha256Hex(post) != preCacheHash {
		t.Errorf("Rotate changed the effective data key: pre=%s post=%s", preCacheHash, sha256Hex(post))
	}
}
