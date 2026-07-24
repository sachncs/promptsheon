// Package models — SystemConfig is the operator-tunable runtime
// setting value type. The settings layer is a thin key/value
// store backed by the `system_config` table; the value is
// JSON-encoded so each key can carry structured data (e.g. an
// OTel resource attribute map) without a schema migration per
// addition.
//
// The CRDT fields (ReplicaID, VersionVector, Tombstone, WriteTS)
// back the last-write-wins register per key. Replicas converge
// to the same SystemConfig row after exchanging rows; the
// merge semantics live in internal/settings/crdt.go.
package models

import "time"

// SystemConfig is one row of the operator-tunable settings
// table. Keys are dotted paths (`otl.endpoint`,
// `llm.openai.api_key_ref`, ...); values are JSON-encoded so
// the schema is implicit per key.
//
// ReplicaID is the writer's per-process CRDT id (a 32-char
// hex string derived from crypto/rand). VersionVector maps
// replica id -> observed-write count; it grows on every Set
// from the local replica and lets a remote merge detect
// "newer than what I saw" without trusting wall-clock time.
// Tombstone is true for deleted keys (see settings.Delete);
// Get/List hide tombstoned rows.
type SystemConfig struct {
	Key           string            `json:"key"`
	Value         string            `json:"value"`
	UpdatedAt     time.Time         `json:"updated_at"`
	UpdatedBy     string            `json:"updated_by"`
	ReplicaID     string            `json:"replica_id"`
	VersionVector map[string]uint64 `json:"version_vector"`
	Tombstone     bool              `json:"tombstone"`
	WriteTS       int64             `json:"write_ts"`
}
