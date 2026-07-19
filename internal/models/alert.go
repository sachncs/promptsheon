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
type WebhookEndpointRecord struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Secret         string    `json:"secret,omitempty"`
	AllowInsecure  bool      `json:"allow_insecure"`
	AllowPrivate   bool      `json:"allow_private"`
	Events         []string  `json:"events"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}
