// Package postgres implements the banditstore.Backend contract
// against a Postgres connection. The schema lives in
// internal/store/migrations/postgres/026_bandit.sql (M3.5
// follow-on per ADR-0019).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sachncs/promptsheon/internal/bandit"
)

// snapshotToRow extracts the exact (alpha, beta) counts from a
// bandit.ArmPosterior for persistence. bandit.ArmPosterior stores
// alpha = successes + 1, beta = failures + 1 (uniform Beta(1,1)
// prior), so we subtract 1 from each side to persist only the
// observation counts; the prior is reapplied on load.
func snapshotToRow(p *bandit.ArmPosterior) (alpha, beta float64) {
	return p.Alpha(), p.Beta()
}

// rowToSnapshot reconstructs a bandit.ArmPosterior from persisted
// (alpha, beta). The bandit package stores alpha = successes + 1,
// beta = failures + 1, so the reconstructed posterior matches the
// cold-start shape; new Observe() calls increment from there.
func rowToSnapshot(alpha, beta float64) *bandit.ArmPosterior {
	return bandit.NewArmPosteriorWithCounts(alpha, beta)
}

// Backend is the Postgres-backed banditstore.Backend.
type Backend struct {
	db *sql.DB
	mu sync.Mutex
}

// Open dials the supplied DSN and pings the database.
func Open(ctx context.Context, dsn string) (*Backend, error) {
	if dsn == "" {
		return nil, errors.New("banditstore/postgres: empty DSN")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("banditstore/postgres: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("banditstore/postgres: ping: %w", err)
	}
	return &Backend{db: db}, nil
}

// Close releases the underlying connection pool.
func (b *Backend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	return b.db.Close()
}

// LoadAll reads every arm-posterior row.
func (b *Backend) LoadAll(ctx context.Context) (map[string]bandit.ArmPosterior, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rows, err := b.db.QueryContext(ctx,
		`SELECT arm_id, alpha, beta FROM bandit_arm_posteriors`)
	if err != nil {
		return nil, fmt.Errorf("banditstore/postgres: load: %w", err)
	}
	defer rows.Close()
	out := make(map[string]bandit.ArmPosterior)
	for rows.Next() {
		var id string
		var alpha, beta float64
		if err := rows.Scan(&id, &alpha, &beta); err != nil {
			return nil, fmt.Errorf("banditstore/postgres: scan: %w", err)
		}
		out[id] = *rowToSnapshot(alpha, beta)
	}
	return out, rows.Err()
}

// SaveAll atomically replaces the arm-posterior table.
func (b *Backend) SaveAll(ctx context.Context, posteriors map[string]bandit.ArmPosterior) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("banditstore/postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM bandit_arm_posteriors`); err != nil {
		return fmt.Errorf("banditstore/postgres: clear: %w", err)
	}
	for id, p := range posteriors {
		alpha, beta := snapshotToRow(&p)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO bandit_arm_posteriors (arm_id, alpha, beta) VALUES ($1, $2, $3)`,
			id, alpha, beta); err != nil {
			return fmt.Errorf("banditstore/postgres: insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("banditstore/postgres: commit: %w", err)
	}
	return nil
}
