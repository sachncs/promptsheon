package models

import "time"

// AuditEntry records a single mutation or action in the system.
type AuditEntry struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Action       string         `json:"action"`
	Resource     string         `json:"resource"`
	Details      map[string]any `json:"details"`
	Timestamp    time.Time      `json:"timestamp"`
	PreviousHash string         `json:"previous_hash"`
	EntryHash    string         `json:"entry_hash"`
}

// AuditFilter defines criteria for listing audit entries.
type AuditFilter struct {
	UserID   string
	Resource string
	Action   string
	Since    *time.Time
	Until    *time.Time
	Limit    int
	Offset   int
}
