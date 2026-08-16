package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/models"
)

// SQLite persistence for vault_state.

func (s *SQLite) GetVaultState(ctx context.Context) (*models.VaultState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT kms_key_id, wrapped_data_key, created_at, updated_at
		   FROM vault_state WHERE id = 0`)
	var vs models.VaultState
	err := row.Scan(&vs.KMSKeyID, &vs.WrappedDataKey, &vs.CreatedAt, &vs.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &vs, nil
}

// SaveVaultState persists the singleton vault_state row. Creates
// the row on first use; subsequent calls update it. SEC-10a.

func (s *SQLite) SaveVaultState(ctx context.Context, vs *models.VaultState) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vault_state (id, kms_key_id, wrapped_data_key, created_at, updated_at)
		VALUES (0, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kms_key_id = excluded.kms_key_id,
			wrapped_data_key = excluded.wrapped_data_key,
			updated_at = excluded.updated_at`,
		vs.KMSKeyID, vs.WrappedDataKey, now, now,
	)
	if err != nil {
		return errf.Errorf("save vault state: %w", err)
	}
	vs.CreatedAt = now
	vs.UpdatedAt = now
	return nil
}

// GetWSNextID returns the persisted SSE hub next-id, or 0 if
// none has been stored yet. OBS-LOG-3.
