package store

import (
	"fmt"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"context"
	"database/sql"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for projects.

func (s *SQLite) CreateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.WorkspaceID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (s *SQLite) GetProject(ctx context.Context, id string) (*capability.Project, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id,
	)
	return scanProject(row)
}

func (s *SQLite) ListProjects(ctx context.Context, workspaceID string) ([]*capability.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE workspace_id = ? ORDER BY name`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func scanProject(scanner interface {
	Scan(dest ...any) error
}) (*capability.Project, error) {
	var p capability.Project
	err := scanner.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------
