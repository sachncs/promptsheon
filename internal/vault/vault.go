// Package vault provides AES-256-GCM encryption for storing sensitive data
// like LLM API keys at rest.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// ErrStopped is returned by Encrypt/Decrypt after Stop has been
// called. The Vault retains no plaintext after Stop, so any
// further use is fail-closed.
var ErrStopped = errors.New("vault: stopped")

// Vault encrypts and decrypts data using AES-256-GCM.
//
// After Stop is called, the in-memory key is zeroized and every
// subsequent Encrypt/Decrypt returns ErrStopped. Stop is
// idempotent and concurrency-safe: parallel callers either see a
// live vault or a stopped vault, never a half-swapped state.
//
// Hot reload: Reload validates a candidate 32-byte key in a
// throwaway cipher before swapping it in. On failure (invalid
// hex, wrong length, all-zero bytes, cipher construction error)
// the existing key is retained and the error is returned.
type Vault struct {
	mu      sync.RWMutex
	key     []byte
	stopped atomic.Bool
}

// New creates a vault with the given 32-byte hex-encoded key.
// The key should be set from PROMPTSHEON_VAULT_KEY env var.
// M-16 fix: reject the all-zero key, which AES would otherwise
// accept but which produces ciphertexts that are trivially
// decryptable (the resulting key schedule is also all-zero). The
// previous implementation only checked byte length, which let a
// misconfigured key pass validation.
func New(hexKey string) (*Vault, error) {
	key, err := parseVaultKey(hexKey)
	if err != nil {
		return nil, err
	}
	return &Vault{key: key}, nil
}

// parseVaultKey validates and decodes the hex-encoded master
// key. Shared by New and Reload so both paths apply the same
// rejection rules (all-zero, length, hex).
func parseVaultKey(hexKey string) ([]byte, error) {
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
	return key, nil
}

// Stop zeroizes the in-memory key and marks the Vault stopped.
// Safe to call from any goroutine and any number of times;
// subsequent Encrypt/Decrypt calls return ErrStopped.
//
// The motivation: a process holding the master key in memory is
// a long-lived secret. After Stop returns, the bytes backing the
// key are overwritten so a post-mortem dump (core file,
// /proc/<pid>/mem) does not reveal the active key. Existing
// ciphertexts remain on disk; reload via New is the documented
// recovery path.
func (v *Vault) Stop() {
	if !v.stopped.CompareAndSwap(false, true) {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
}

// Stopped reports whether Stop has been called.
func (v *Vault) Stopped() bool {
	return v.stopped.Load()
}

// Reload atomically swaps the master key for the one encoded in
// hexKey. The candidate key is decoded, validated, and built into
// a throwaway cipher before the swap; on any failure the
// existing key is retained untouched and the error is returned.
//
// Concurrency: callers holding a successful Encrypt/Decrypt
// before Reload finish their operation against the old key;
// callers arriving after Reload use the new key. Ciphertexts
// written under the old key remain decryptable through the
// process that wrote them only if the daemon does not call Reload
// during their lifetime — operators who rotate the master key
// must plan to re-encrypt stored secrets.
func (v *Vault) Reload(hexKey string) error {
	if v.stopped.Load() {
		return ErrStopped
	}
	key, err := parseVaultKey(hexKey)
	if err != nil {
		return err
	}
	// Build the cipher up front so a malformed key is caught
	// before the swap. AES-256 never fails this for a 32-byte
	// key, but the throwaway construction also forces the
	// GCM initialisation, which is the cheaper real failure
	// mode.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}
	if _, err := cipher.NewGCM(block); err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.stopped.Load() {
		return ErrStopped
	}
	// Zeroize the old key before installing the new one.
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = key
	return nil
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
	block, err := v.cipher()
	if err != nil {
		return nil, err
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

	block, err := v.cipher()
	if err != nil {
		return "", err
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

// cipher returns the AES cipher block for the current key, or
// ErrStopped if the Vault has been Stopped. The key slice is
// returned under a read lock so a concurrent Reload cannot
// swap the slice mid-call.
func (v *Vault) cipher() (cipher.Block, error) {
	if v.stopped.Load() {
		return nil, ErrStopped
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.stopped.Load() || v.key == nil {
		return nil, ErrStopped
	}
	return aes.NewCipher(v.key)
}
