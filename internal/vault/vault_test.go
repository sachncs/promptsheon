package vault

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "my-secret-api-key-12345"
	encrypted, err := v.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if encrypted == plaintext {
		t.Fatal("encrypted text should differ from plaintext")
	}

	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	v1, _ := New(key1)
	v2, _ := New(key2)

	encrypted, _ := v1.Encrypt("secret")
	_, err := v2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestNewInvalidKey(t *testing.T) {
	// Too short
	_, err := New("0123456789abcdef")
	if err == nil {
		t.Fatal("expected error for short key")
	}

	// Invalid hex
	_, err = New("not-valid-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, _ := New(key)

	enc1, _ := v.Encrypt("same plaintext")
	enc2, _ := v.Encrypt("same plaintext")

	if enc1 == enc2 {
		t.Fatal("encryption should produce different ciphertext due to random nonce")
	}

	// Both should decrypt to same plaintext
	d1, _ := v.Decrypt(enc1)
	d2, _ := v.Decrypt(enc2)
	if d1 != d2 || d1 != "same plaintext" {
		t.Fatal("both should decrypt to same plaintext")
	}
}
