// Package models provides data models for persistence.
package models

import "time"

// AlertRuleRecord represents a persisted alert rule.
type AlertRuleRecord struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Severity  string         `json:"severity"`
	Enabled   bool           `json:"enabled"`
	Threshold float64        `json:"threshold"`
	Duration  int            `json:"duration_minutes"`
	Window    int            `json:"window_minutes"`
	Config    map[string]any `json:"config,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// AlertRecord represents a persisted alert.
type AlertRecord struct {
	ID          string         `json:"id"`
	RuleID      string         `json:"rule_id"`
	RuleName    string         `json:"rule_name"`
	Severity    string         `json:"severity"`
	Status      string         `json:"status"`
	Message     string         `json:"message"`
	Details     map[string]any `json:"details,omitempty"`
	TriggeredAt time.Time      `json:"triggered_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
}

// NotificationGroupRecord represents a persisted notification group.
type NotificationGroupRecord struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// WebhookEndpointRecord represents a persisted webhook subscription.
// Mirrors webhook.Endpoint but lives in the models package so the store
// does not import the webhook package (which would create a cycle).
//
// Secret is a credential; it is marked json:"-" so generic JSON
// encoders (audit log, error responses, debug dumps) cannot leak
// the webhook HMAC key. The plaintext form is held only
// transiently in memory and never persisted; the database stores
// the AES-GCM ciphertext (SecretCiphertext). Endpoints that need
// to surface the ciphertext over the wire use a dedicated DTO.
//
// SEC-11 / SEC-4: there is no AllowInsecure / AllowPrivate flag.
// HTTPS is mandatory at registration; loopback / private /
// link-local destinations are refused at registration and at
// every delivery. There is no opt-in.
type WebhookEndpointRecord struct {
	ID               string    `json:"id"`
	URL              string    `json:"url"`
	Secret           string    `json:"-"`
	SecretCiphertext []byte    `json:"-"`
	Events           []string  `json:"events"`
	Active           bool      `json:"active"`
	CreatedAt        time.Time `json:"created_at"`
}
