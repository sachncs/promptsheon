package models

import "time"

// VaultState is the singleton row in the vault_state table
// (id = 0, enforced via CHECK). It holds the KMS-wrapped data
// key so the process can recover the plaintext by calling
// KMSClient.Decrypt on the persisted ciphertextBlob.
//
// The wrapped_data_key is the CiphertextBlob returned by
// GenerateDataKey; the plaintext it unwraps to is the AES-256
// data key that encrypts every secret in provider_keys. Storing
// the wrapped form (not the plaintext) keeps secrets encrypted
// at rest and survives process restarts.
//
// SEC-10a.
type VaultState struct {
	KMSKeyID       string    `json:"kms_key_id"`
	WrappedDataKey []byte    `json:"wrapped_data_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
