// Package models — SystemConfig is the operator-tunable runtime
// setting value type. The settings layer is a thin key/value
// store backed by the `system_config` table; the value is
// JSON-encoded so each key can carry structured data (e.g. an
// OTel resource attribute map) without a schema migration per
// addition.
package models

import "time"

// SystemConfig is one row of the operator-tunable settings
// table. Keys are dotted paths (`otl.endpoint`,
// `llm.openai.api_key_ref`, ...); values are JSON-encoded so
// the schema is implicit per key.
type SystemConfig struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}
