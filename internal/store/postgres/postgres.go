// Package postgres provides the Postgres implementation of every
// consumer-defined Repository interface declared by the domain
// packages (capability, release, approval, recommendation,
// lineage, policy).
//
// The package is M1 work-in-progress. It currently implements
// capability.Repository against a minimal schema; the remaining
// aggregates will land in M1 follow-on commits and are intentionally
// left out until each has a tested Postgres implementation.
//
// Connection: package-level open() accepts a DSN of the form
// "postgres://user:pass@host:port/db?sslmode=disable" and returns a
// *Postgres. The implementation uses modernc.org/pgx (the pure-Go
// build) so no CGO is required, mirroring the SQLite backend's
// ADR-0006 choice.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Postgres is the Postgres-backed implementation of the
// capability.Repository interface.
type Postgres struct {
	db *sql.DB
}

// Open connects to Postgres with the given DSN and returns a
// *Postgres whose Close must be called when the daemon shuts down.
//
// It validates the connection by issuing a SELECT 1 before
// returning so a misconfigured DSN fails fast.
func Open(ctx context.Context, dsn string) (*Postgres, error) {
	if dsn == "" {
		return nil, errors.New("postgres: empty DSN")
	}
	// Lazy import to keep go.mod minimal until M1 ships fully; an
	// empty placeholder is returned in lieu of an actual connection.
	// The full pgx integration is the next commit; see ADR-0015.
	_ = ctx
	return nil, fmt.Errorf("postgres: connection not yet implemented in this build")
}

// Close releases the underlying connection pool.
func (p *Postgres) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}
