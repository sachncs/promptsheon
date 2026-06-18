package models

import "time"

// APIKey represents an API key for authenticating with the Promptsheon API.
type APIKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"`          // never serialized
	KeyPrefix string     `json:"key_prefix"` // first 8 chars for display
	Role      string     `json:"role"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Revoked   bool       `json:"revoked"`
}
