package models

import "time"

// ProviderKey stores an encrypted LLM API key for a provider.
type ProviderKey struct {
	ID           string    `json:"id"`
	ProviderName string    `json:"provider_name"`
	KeyName      string    `json:"key_name"`
	EncryptedKey string    `json:"encrypted_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
