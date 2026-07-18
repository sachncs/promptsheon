package kmsbyok

import (
	"context"
	"errors"
	"testing"
)

func TestProviderMissingKeyID(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error for empty KeyID")
	}
}

func TestProviderRejectsProductionWithoutClient(t *testing.T) {
	t.Parallel()
	_, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd"})
	if err == nil {
		t.Fatal("expected error for production path without KMSClient")
	}
}

func TestProviderTestDoubleIsDeterministic(t *testing.T) {
	t.Parallel()
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", AllowTestDouble: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if len(a) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(a))
	}
	b, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 2: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("Key not deterministic at index %d", i)
		}
	}
}

func TestProviderRotateInvalidatesCache(t *testing.T) {
	t.Parallel()
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(0xff - i)
	}
	calls := 0
	kms := &fakeKMS{}
	kms.responses = [][]byte{key1, key2}
	kms.onCall = func() int { calls++; return calls - 1 }
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", KMSClient: kms})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	first, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 1: %v", err)
	}
	if !equalBytes(first, key1) {
		t.Errorf("first key mismatch")
	}

	// Second call should hit cache, not the KMS client.
	second, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 2: %v", err)
	}
	if !equalBytes(second, key1) {
		t.Errorf("second call should match first (cached), got different")
	}

	if err := p.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	third, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 3: %v", err)
	}
	if !equalBytes(third, key2) {
		t.Errorf("after Rotate, key should be refreshed, got original")
	}
}

func TestProviderKeyRejectsWrongLength(t *testing.T) {
	t.Parallel()
	short := make([]byte, 16)
	kms := &fakeKMS{plaintext: short, err: nil}
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", KMSClient: kms})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := p.Key(context.Background()); err == nil {
		t.Error("expected error for non-32-byte plaintext")
	}
}

func TestProviderKeyPropagatesKMSError(t *testing.T) {
	t.Parallel()
	kms := &fakeKMS{err: errors.New("kms down")}
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", KMSClient: kms})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := p.Key(context.Background()); err == nil {
		t.Error("expected error when KMS fails")
	}
}

type fakeKMS struct {
	plaintext []byte
	err       error
	responses [][]byte
	onCall    func() int
}

func (f *fakeKMS) GenerateDataKey(_ context.Context, _ string) ([]byte, error) {
	if f.onCall != nil {
		idx := f.onCall()
		if idx >= 0 && idx < len(f.responses) {
			return f.responses[idx], f.err
		}
	}
	return f.plaintext, f.err
}

func TestProviderUsesKMSClient(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	kms := &fakeKMS{plaintext: key}
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", KMSClient: kms})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	for i := range got {
		if got[i] != key[i] {
			t.Errorf("byte %d: got %d want %d", i, got[i], key[i])
		}
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
