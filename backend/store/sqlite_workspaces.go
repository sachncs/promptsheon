package store
import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sachncs/promptsheon/backend/errs"
	"github.com/sachncs/promptsheon/backend/capability"
)

// SQLite persistence for workspaces.

func (s *SQLite) CreateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, organization, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Organization, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}


func (s *SQLite) GetWorkspace(ctx context.Context, id string) (*capability.Workspace, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces WHERE id = ?`, id,
	)
	return scanWorkspace(row)
}


func (s *SQLite) ListWorkspaces(ctx context.Context) ([]*capability.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}


func (s *SQLite) UpdateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ?`,
		w.Name, w.Organization, w.UpdatedAt, w.ID,
	)
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	return nil
}


func (s *SQLite) DeleteWorkspace(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}


func scanWorkspace(scanner interface {
	Scan(dest ...any) error
}) (*capability.Workspace, error) {
	var w capability.Workspace
	err := scanner.Scan(&w.ID, &w.Name, &w.Organization, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}
	return &w, nil
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------


