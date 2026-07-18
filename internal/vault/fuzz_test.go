package vault

import (
	"encoding/hex"
	"testing"
)

func FuzzVaultEncryptDecrypt(f *testing.F) {
	f.Add([]byte("hello world"))
	f.Add([]byte(""))
	f.Add([]byte("\x00\x01\x02"))

	f.Fuzz(func(t *testing.T, plaintext []byte) {
		if len(plaintext) > 4096 {
			t.Skip("oversized input")
		}
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		v, err := New(hex.EncodeToString(key))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ct, err := v.Encrypt(string(plaintext))
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if len(ct) == 0 {
			t.Fatal("empty ciphertext")
		}
		pt, err := v.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if pt != string(plaintext) {
			t.Errorf("round-trip mismatch: %q != %q", pt, plaintext)
		}
	})
}
