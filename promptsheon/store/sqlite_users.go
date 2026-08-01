package store

import (
	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"context"
	"database/sql"
	"errors"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for users.

func (s *SQLite) CreateUser(ctx context.Context, u *models.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return errf.Errorf("insert user: %w", err)
	}
	return nil
}

// BootstrapAdmin atomically inserts the first admin user and
// returns errs.ErrStoreConflict if any non-system user row already
// exists. The system user 'api' (seeded by migration 057) is
// ignored so the bootstrap endpoint stays available even when
// the audit FK has been satisfied.
//
// BootstrapAdmin is the single-caller bootstrap. SEC-5a: 100
// concurrent POST /api/v1/setup calls must produce exactly one
// admin key, with the rest seeing errs.ErrStoreConflict. The race-free
// path is INSERT ... ON CONFLICT (email) DO NOTHING: SQLite
// resolves the conflict at write time, so even under a
// DEFERRED transaction the second writer's INSERT silently
// drops and RowsAffected returns 0. We then check the rows-
// affected count and return errs.ErrStoreConflict for the loser.
//
// The previous implementation used a SELECT COUNT(*) then a
// plain INSERT; under DEFERRED locking both writers could read
// the same empty count and both insert successfully, minting
// two admin keys.

func (s *SQLite) BootstrapAdmin(ctx context.Context, u *models.User, key *models.APIKey) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errf.Errorf("bootstrap begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (email) DO NOTHING`,
		u.ID, u.Email, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return errf.Errorf("bootstrap insert user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return errf.Errorf("bootstrap rows affected: %w", err)
	}
	if n == 0 {
		// Another caller won the race. Roll back the key insert
		// (we never get there) and surface a typed conflict.
		return errs.ErrStoreConflict
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.UserID, key.Name, key.KeyHash, key.KeyPrefix, key.Role, key.CreatedAt,
	); err != nil {
		return errf.Errorf("bootstrap insert key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return errf.Errorf("bootstrap commit: %w", err)
	}
	return nil
}

func (s *SQLite) GetUser(ctx context.Context, id string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users WHERE id = ?`, id,
	)
	return scanUser(row)
}

func (s *SQLite) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users WHERE email = ?`, email,
	)
	return scanUser(row)
}

func (s *SQLite) ListUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, errf.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLite) UpdateUser(ctx context.Context, u *models.User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errf.Errorf("update user begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Look up the existing role so we only revoke API keys when
	// the role actually changed. Without this, every PUT on the
	// user record would invalidate live keys.
	var oldRole string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, u.ID).Scan(&oldRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errf.Errorf("user not found: %s", u.ID)
		}
		return errf.Errorf("update user lookup: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE users SET email=?, name=?, role=?, updated_at=? WHERE id=?`,
		u.Email, u.Name, u.Role, u.UpdatedAt, u.ID,
	)
	if err != nil {
		return errf.Errorf("update user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errf.Errorf("user not found: %s", u.ID)
	}

	// SEC-6: when the user's role changes (typically a demotion
	// from admin to reader), revoke every non-expired, non-revoked
	// API key issued to that user. The holder's existing session
	// tokens stop working on the next request because the
	// authenticator re-reads the role on every call.
	if oldRole != u.Role {
		if _, err := tx.ExecContext(ctx,
			`UPDATE api_keys SET revoked = 1 WHERE user_id = ? AND revoked = 0`,
			u.ID,
		); err != nil {
			return errf.Errorf("revoke stale api keys: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errf.Errorf("update user commit: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errf.Errorf("delete user begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET revoked = 1 WHERE user_id = ? AND revoked = 0`, id); err != nil {
		return errf.Errorf("revoke keys on delete: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return errf.Errorf("delete user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errf.Errorf("user not found: %s", id)
	}
	if err := tx.Commit(); err != nil {
		return errf.Errorf("delete user commit: %w", err)
	}
	return nil
}

func scanUser(row scannable) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrStoreNotFound
		}
		return nil, errf.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func scanUserRow(rows *sql.Rows) (*models.User, error) {
	return scanUser(rows)
}

// --- API Keys ---
