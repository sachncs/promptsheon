package store

import (
	"fmt"
	"context"
	"database/sql"
	"errors"
)

// SQLite persistence for ws_state.

func (s *SQLite) GetWSNextID(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT next_id FROM ws_state WHERE id = 0`,
	).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get ws next id: %w", err)
	}
	return n, nil
}

// SetWSNextID persists the SSE hub's next-id so a restart can
// continue from the same point. OBS-LOG-3.

func (s *SQLite) SetWSNextID(ctx context.Context, n int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ws_state (id, next_id, updated_at)
		VALUES (0, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			next_id = excluded.next_id,
			updated_at = excluded.updated_at`,
		n,
	)
	if err != nil {
		return fmt.Errorf("set ws next id: %w", err)
	}
	return nil
}

// GetEnforcerBudget removed (c2.22). The PersistedEnforcer was
// dropped; SetBudget / SetQuota are now in-memory only.
