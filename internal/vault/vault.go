// Package vault provides AES-256-GCM encryption for storing sensitive data
// like LLM API keys at rest.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Vault encrypts and decrypts data using AES-256-GCM.
type Vault struct {
	key []byte
}

// New creates a vault with the given 32-byte hex-encoded key.
// The key should be set from PROMPTSHEON_VAULT_KEY env var.
// M-16 fix: reject the all-zero key, which AES would otherwise
// accept but which produces ciphertexts that are trivially
// decryptable (the resulting key schedule is also all-zero). The
// previous implementation only checked byte length, which let a
// misconfigured key pass validation.
func New(hexKey string) (*Vault, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode vault key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("vault key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, fmt.Errorf("vault key is all zeros; refusing to use a trivially-decryptable key")
	}
	return &Vault{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a hex-encoded string.
func (v *Vault) Encrypt(plaintext string) (string, error) {
	ct, err := v.EncryptBytes([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ct), nil
}

// EncryptBytes encrypts raw bytes using AES-256-GCM and returns
// the nonce-prefixed ciphertext as a byte slice. Useful for
// callers that want to store the result in a BLOB column
// rather than a TEXT column.
func (v *Vault) EncryptBytes(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a hex-encoded AES-256-GCM ciphertext.
func (v *Vault) Decrypt(hexCiphertext string) (string, error) {
	ciphertext, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
