package vault

import (
	"encoding/hex"
	"testing"
)

// BenchmarkEncrypt exercises the AES-256-GCM hot path on a
// realistic blob size. Use to track per-call allocations.
func BenchmarkEncrypt(b *testing.B) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	v, err := New(hex.EncodeToString(key))
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	plaintext := make([]byte, 4096)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.Encrypt(string(plaintext)); err != nil {
			b.Fatalf("Encrypt: %v", err)
		}
	}
}

// BenchmarkEncryptDecrypt exercises the full round-trip so the
// total cost per stored secret is captured.
func BenchmarkEncryptDecrypt(b *testing.B) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	v, err := New(hex.EncodeToString(key))
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	plaintext := make([]byte, 1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := v.Encrypt(string(plaintext))
		if err != nil {
			b.Fatalf("Encrypt: %v", err)
		}
		if _, err := v.Decrypt(ct); err != nil {
			b.Fatalf("Decrypt: %v", err)
		}
	}
}
