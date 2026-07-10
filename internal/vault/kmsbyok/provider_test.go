package kmsbyok

import (
	"context"
	"testing"
)

func TestProviderMissingKeyID(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error for empty KeyID")
	}
}

func TestProviderTestDoubleIsDeterministic(t *testing.T) {
	t.Parallel()
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd"})
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

type fakeKMS struct {
	plaintext []byte
	err       error
}

func (f *fakeKMS) GenerateDataKey(_ context.Context, _ string) ([]byte, error) {
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
