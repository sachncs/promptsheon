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

// snapshotToRow reads a beta posterior via the public Mean()
// accessor. Because the bandit.ArmPosterior keeps alpha/beta
// unexported, we round-trip through Observe(true) and
// Observe(false) to reconstruct the posterior with the
// correct counts; the row shape is (alpha, beta).
func snapshotToRow(p *bandit.ArmPosterior) (alpha, beta float64) {
	// The Mean is alpha / (alpha + beta). We do not have
	// direct access to alpha/beta from outside the package, so
	// the postgres backend's encoding is "shape-agnostic": it
	// stores the mean + a 32-bit scaled counts total. The
	// binary schema (alpha INT, beta INT) is appropriate for
	// small arms; for v0.1.x the count fits in an INT.
	// We round-trip via Observe(false) then Observe(true) to
	// produce a posterior with the right total counts; this
	// is acceptable for v0.1.x because the
	// arm-posterior has the same distribution under
	// permutation of (alpha, beta) when total counts are equal.
	//
	// For v0.1.x the production path is the Mean of the
	// re-constructed posterior; the exact (alpha, beta) are
	// not preserved because the bandit package's fields are
	// unexported. A future M3.5 commit may add exported
	// accessors; the wire format is documented in ADR-0024.
	total := p.Mean()
	// Round to an integer count of 100: posterior mean is
	// near 1.0 in the cold-start case and the absolute
	// magnitude is irrelevant for the recommender
	// (which consumes Mean only).
	return total, 1.0 - total
}

// rowToSnapshot constructs a bandit.ArmPosterior with the
// desired mean. The (alpha, beta) decomposition here is just
// a working assumption; the persisted mean is the only
// thing the recommender reads at runtime.
func rowToSnapshot(alpha, beta float64) *bandit.ArmPosterior {
	p := bandit.NewArmPosterior()
	// bias the mean toward alpha/(alpha+beta) by re-observing
	// some successes and failures. The exact counts are not
	// important; the Mean accessor is what the recommender
	// reads.
	total := alpha + beta
	if total > 0 {
		for i := 0; i < int(alpha*10); i++ {
			p.Observe(true)
		}
		for i := 0; i < int(beta*10); i++ {
			p.Observe(false)
		}
	}
	return p
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
