package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/models"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for apikeys.

func (s *SQLite) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	const q = `INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		key.ID, key.UserID, key.Name, key.KeyHash, key.KeyPrefix,
		key.Role, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return errf.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLite) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	const sqlText = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE key_hash = ? AND revoked = 0`
	var k models.APIKey
	var err error
	// PERF-DB-1: use the prepared statement when available.
	if s.stmtGetAPIKeyByHash != nil {
		err = s.stmtGetAPIKeyByHash.QueryRowContext(ctx, keyHash).Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
		)
	} else {
		err = s.db.QueryRowContext(ctx, sqlText, keyHash).Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
		)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errf.Errorf("get api key by hash: %w", err)
	}
	return &k, nil
}

func (s *SQLite) GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE id = ?`
	var k models.APIKey
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, errf.Errorf("get api key by id: %w", err)
	}
	return &k, nil
}

func (s *SQLite) DeleteAPIKey(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET revoked = 1 WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return errf.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *SQLite) ListAPIKeysByUser(ctx context.Context, userID string) ([]*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, errf.Errorf("list api keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
		); err != nil {
			return nil, errf.Errorf("scan api key: %w", err)
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (s *SQLite) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET last_used = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, time.Now(), id)
	if err != nil {
		return errf.Errorf("update api key last used: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Provider Keys
// ---------------------------------------------------------------------------
