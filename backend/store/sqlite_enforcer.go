package store
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SQLite persistence for enforcer.

func (s *SQLite) GetEnforcerBudget(ctx context.Context, workspaceID string) ([]byte, error) {
	return s.getEnforcerPayload(ctx, workspaceID, "budget")
}

// SetEnforcerBudget persists the budget payload for a workspace.
// OBS-13.

func (s *SQLite) SetEnforcerBudget(ctx context.Context, workspaceID string, payload []byte) error {
	return s.setEnforcerPayload(ctx, workspaceID, "budget", payload)
}

// GetEnforcerQuota returns the persisted quota payload for a
// workspace, or nil if none. OBS-13.

func (s *SQLite) GetEnforcerQuota(ctx context.Context, workspaceID string) ([]byte, error) {
	return s.getEnforcerPayload(ctx, workspaceID, "quota")
}

// SetEnforcerQuota persists the quota payload for a workspace.
// OBS-13.

func (s *SQLite) SetEnforcerQuota(ctx context.Context, workspaceID string, payload []byte) error {
	return s.setEnforcerPayload(ctx, workspaceID, "quota", payload)
}


func (s *SQLite) getEnforcerPayload(ctx context.Context, workspaceID, kind string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM enforcer_state WHERE workspace_id = ? AND kind = ?`,
		workspaceID, kind,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enforcer %s for %s: %w", kind, workspaceID, err)
	}
	return payload, nil
}


func (s *SQLite) setEnforcerPayload(ctx context.Context, workspaceID, kind string, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO enforcer_state (workspace_id, kind, payload, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(workspace_id, kind) DO UPDATE SET
			payload = excluded.payload,
			updated_at = excluded.updated_at`,
		workspaceID, kind, payload,
	)
	if err != nil {
		return fmt.Errorf("set enforcer %s for %s: %w", kind, workspaceID, err)
	}
	return nil
}

// GetSystemConfig returns the CRDT record for one key. If the
// row doesn't exist, sql.ErrNoRows is returned; the settings
// resolver treats ErrNoRows as "use the env default".
//
// The full record (including the tombstone flag, version
// vector, replica id, and monotonic timestamp) is returned so
// downstream merges see the exact LWW state.

