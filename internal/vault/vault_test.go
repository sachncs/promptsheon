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

	v1, err := New(key1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := New(key2)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := v1.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = v2.Decrypt(encrypted)
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
			name:    "all zeros",
			key:     "0000000000000000000000000000000000000000000000000000000000000000",
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
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	enc1, err := v.Encrypt("same plaintext")
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := v.Encrypt("same plaintext")
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Fatal("encryption should produce different ciphertext due to random nonce")
	}

	// Both should decrypt to same plaintext
	d1, err := v.Decrypt(enc1)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := v.Decrypt(enc2)
	if err != nil {
		t.Fatal(err)
	}
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

func TestDecryptInvalidHex(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.Decrypt("this-is-not-hex!!")
	if err == nil {
		t.Fatal("expected error for invalid hex input")
	}
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.Decrypt("ab")
	if err == nil {
		t.Fatal("expected error for ciphertext too short")
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
