package models

import "time"

// ProviderKey stores an encrypted LLM API key for a provider.
//
// EncryptedKey is the AES-GCM ciphertext of the upstream API
// key. It is marked json:"-" so generic JSON encoders (audit
// logs, error responses, debug dumps) cannot leak the ciphertext;
// a stray log line of a ProviderKey row would still be enough
// to start a brute-force search for the underlying key.
type ProviderKey struct {
	ID           string    `json:"id"`
	ProviderName string    `json:"provider_name"`
	KeyName      string    `json:"key_name"`
	EncryptedKey string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
