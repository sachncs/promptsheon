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
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "too short",
			key:     "0123456789abcdef",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			key:     "not-valid-hex",
			wantErr: true,
		},
		{
			name:    "valid key",
			key:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

func TestEncryptDecryptEmptyString(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := ""
	encrypted, err := v.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecryptLongString(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "This is a longer string that contains multiple words and special characters! @#$%^&*()"
	encrypted, err := v.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}
