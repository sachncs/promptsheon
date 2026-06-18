// Package snapshot provides output snapshot persistence for LLM calls.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Snapshot captures an LLM input/output pair for reproducibility and debugging.
type Snapshot struct {
	ID            string            `json:"id"`
	PromptHash    string            `json:"prompt_hash"`
	PromptText    string            `json:"prompt_text"`
	Model         string            `json:"model"`
	ResponseText  string            `json:"response_text"`
	Provider      string            `json:"provider"`
	TokenUsage    TokenUsage        `json:"token_usage"`
	LatencyMs     int64             `json:"latency_ms"`
	Hallucination float64           `json:"hallucination_score"`
	Metadata      map[string]string `json:"metadata"`
	CreatedAt     time.Time         `json:"created_at"`
}

// TokenUsage records token counts for a snapshot.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Filter defines criteria for listing snapshots.
type Filter struct {
	PromptHash string
	Model      string
	Limit      int
	Offset     int
}

// Store persists snapshots in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore creates a snapshot store backed by the given database.
func NewStore(db *sql.DB) (*Store, error) {
	migration := `
	CREATE TABLE IF NOT EXISTS output_snapshots (
		id TEXT PRIMARY KEY,
		prompt_hash TEXT NOT NULL,
		prompt_text TEXT NOT NULL,
		model TEXT NOT NULL,
		response_text TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		token_usage TEXT NOT NULL DEFAULT '{}',
		latency_ms INTEGER NOT NULL DEFAULT 0,
		hallucination_score REAL NOT NULL DEFAULT 0,
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_snapshots_prompt ON output_snapshots(prompt_hash);
	CREATE INDEX IF NOT EXISTS idx_snapshots_model ON output_snapshots(model);
	CREATE INDEX IF NOT EXISTS idx_snapshots_created ON output_snapshots(created_at);
	`
	if _, err := db.Exec(migration); err != nil {
		return nil, fmt.Errorf("migrate snapshots: %w", err)
	}
	return &Store{db: db}, nil
}

// Save persists a snapshot.
func (s *Store) Save(ctx context.Context, snap *Snapshot) error {
	usageJSON, err := json.Marshal(snap.TokenUsage)
	if err != nil {
		usageJSON = []byte("{}")
	}
	metaJSON, err := json.Marshal(snap.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO output_snapshots
		(id, prompt_hash, prompt_text, model, response_text, provider, token_usage, latency_ms, hallucination_score, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.PromptHash, snap.PromptText, snap.Model, snap.ResponseText,
		snap.Provider, string(usageJSON), snap.LatencyMs, snap.Hallucination,
		string(metaJSON), snap.CreatedAt,
	)
	return err
}

// List returns snapshots matching the filter.
func (s *Store) List(ctx context.Context, f Filter) ([]*Snapshot, error) {
	query := `SELECT id, prompt_hash, prompt_text, model, response_text, provider,
		token_usage, latency_ms, hallucination_score, metadata, created_at
		FROM output_snapshots WHERE 1=1`
	args := []any{}

	if f.PromptHash != "" {
		query += " AND prompt_hash = ?"
		args = append(args, f.PromptHash)
	}
	if f.Model != "" {
		query += " AND model = ?"
		args = append(args, f.Model)
	}

	query += " ORDER BY created_at DESC"

	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}
	if f.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, f.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*Snapshot
	for rows.Next() {
		snap := &Snapshot{}
		var usageJSON, metaJSON string
		if err := rows.Scan(&snap.ID, &snap.PromptHash, &snap.PromptText, &snap.Model,
			&snap.ResponseText, &snap.Provider, &usageJSON, &snap.LatencyMs,
			&snap.Hallucination, &metaJSON, &snap.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(usageJSON), &snap.TokenUsage) //nolint:errcheck
		json.Unmarshal([]byte(metaJSON), &snap.Metadata)    //nolint:errcheck
		snaps = append(snaps, snap)
	}
	return snaps, nil
}

// Get retrieves a snapshot by ID.
func (s *Store) Get(ctx context.Context, id string) (*Snapshot, error) {
	snap := &Snapshot{}
	var usageJSON, metaJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, prompt_hash, prompt_text, model, response_text, provider,
		token_usage, latency_ms, hallucination_score, metadata, created_at
		FROM output_snapshots WHERE id = ?`, id,
	).Scan(&snap.ID, &snap.PromptHash, &snap.PromptText, &snap.Model,
		&snap.ResponseText, &snap.Provider, &usageJSON, &snap.LatencyMs,
		&snap.Hallucination, &metaJSON, &snap.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(usageJSON), &snap.TokenUsage) //nolint:errcheck
	json.Unmarshal([]byte(metaJSON), &snap.Metadata)    //nolint:errcheck
	return snap, nil
}
