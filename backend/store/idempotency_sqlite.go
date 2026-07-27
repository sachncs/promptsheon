package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// IdempotencyStore persists idempotency cache entries so multi-
// replica deployments see the same replay. API-IDEMP-1.
//
// The interface lives in the store package (not the api
// package) to avoid a circular dependency: the api package
// imports the store, never the other way around.
type IdempotencyStore interface {
	GetIdempotency(ctx context.Context, key string) (IdempotencyEntry, error)
	PutIdempotency(ctx context.Context, key string, entry IdempotencyEntry) error
}

// IdempotencyEntry is the persisted shape. Fields are exported
// so the api package can decode them without going through a
// store-specific adapter.
type IdempotencyEntry struct {
	Expires    time.Time
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// ErrIdempotencyMiss signals a cache miss to the middleware.
var ErrIdempotencyMiss = errors.New("idempotency: miss")

// SQLiteIdempotencyStore is the default implementation, backed
// by the `idempotency_cache` table created in migration 013.
type SQLiteIdempotencyStore struct {
	db *sql.DB
}

func NewSQLiteIdempotencyStore(db *sql.DB) *SQLiteIdempotencyStore {
	return &SQLiteIdempotencyStore{db: db}
}

// GetIdempotency returns the cached response for key, or
// ErrIdempotencyMiss if the key is unknown or expired. A
// background DELETE on expiry keeps the table small; the
// current Get path also lazy-deletes expired rows so the
// replay window is correct even before the janitor runs.
func (s *SQLiteIdempotencyStore) GetIdempotency(ctx context.Context, key string) (IdempotencyEntry, error) {
	var (
		expires    time.Time
		statusCode int
		headerJSON string
		body       []byte
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT expires_at, status_code, headers, body
		FROM idempotency_cache
		WHERE key = ?`, key).Scan(&expires, &statusCode, &headerJSON, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return IdempotencyEntry{}, ErrIdempotencyMiss
	}
	if err != nil {
		return IdempotencyEntry{}, fmt.Errorf("idempotency get: %w", err)
	}
	if time.Now().After(expires) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM idempotency_cache WHERE key = ?`, key)
		return IdempotencyEntry{}, ErrIdempotencyMiss
	}
	var headers http.Header
	if err := json.Unmarshal([]byte(headerJSON), &headers); err != nil {
		return IdempotencyEntry{}, fmt.Errorf("idempotency decode headers: %w", err)
	}
	return IdempotencyEntry{
		Expires:    expires,
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

// PutIdempotency writes the entry, replacing any existing row
// with the same key. UPSERT keeps the operation O(1) on
// repeated retries of the same key.
func (s *SQLiteIdempotencyStore) PutIdempotency(ctx context.Context, key string, entry IdempotencyEntry) error {
	headerJSON, err := json.Marshal(entry.Headers)
	if err != nil {
		return fmt.Errorf("idempotency encode headers: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO idempotency_cache (key, expires_at, status_code, headers, body)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			expires_at  = excluded.expires_at,
			status_code = excluded.status_code,
			headers     = excluded.headers,
			body        = excluded.body`,
		key, entry.Expires, entry.StatusCode, string(headerJSON), entry.Body)
	if err != nil {
		return fmt.Errorf("idempotency put: %w", err)
	}
	return nil
}
