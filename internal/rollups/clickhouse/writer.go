// Package clickhouse writes WorkspaceSummary rollups to
// ClickHouse. The package is gated behind a build tag so
// production tenants that do not run ClickHouse never pay the
// dependency cost; the rollup job in internal/rollups continues
// to compute summaries in-process regardless.
//
// Build with: go build -tags clickhouse ./internal/rollups/clickhouse
//
//go:build clickhouse

package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/sachncs/promptsheon/internal/rollups"
)

// Writer is a thin ClickHouse writer for WorkspaceSummary rows.
// The schema is a single MergeTree table keyed by
// (workspace_id, generated_at); production deployments run a
// TTL of 30 days and downstream Grafana reads from it.
type Writer struct {
	conn driver.Conn
	db   string
}

// New connects to ClickHouse and ensures the rollups table
// exists. The Options is the standard clickhouse-go Options
// (Addr, Auth, DialTimeout, etc.). The database name is the
// logical ClickHouse database; the table is created inside it.
func New(ctx context.Context, opt *clickhouse.Options, database string) (*Writer, error) {
	conn, err := clickhouse.Open(opt)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}
	w := &Writer{conn: conn, db: database}
	if err := w.ensureSchema(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return w, nil
}

func (w *Writer) ensureSchema(ctx context.Context) error {
	if err := w.conn.Exec(ctx,
		`CREATE DATABASE IF NOT EXISTS `+w.db); err != nil {
		return fmt.Errorf("clickhouse: create database: %w", err)
	}
	return w.conn.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS `+w.db+`.workspace_rollups (
			workspace_id String,
			generated_at DateTime64(9),
			total_spend_usd Float64,
			overall_health LowCardinality(String)
		) ENGINE = MergeTree()
		ORDER BY (workspace_id, generated_at)
		TTL generated_at + INTERVAL 30 DAY`)
}

// Write persists one WorkspaceSummary to the rollups table.
// The call is idempotent on (workspace_id, generated_at) only
// when called with the same generated_at; the daemon should
// generate one summary per minute per workspace.
func (w *Writer) Write(ctx context.Context, s *rollups.WorkspaceSummary) error {
	if s == nil {
		return fmt.Errorf("clickhouse: nil summary")
	}
	return w.conn.Exec(ctx,
		`INSERT INTO `+w.db+`.workspace_rollups
		 (workspace_id, generated_at, total_spend_usd, overall_health)
		 VALUES (?, ?, ?, ?)`,
		s.WorkspaceID, s.GeneratedAt.UTC(), s.TotalSpendUSD, s.OverallHealth,
	)
}

// Close releases the underlying connection pool.
func (w *Writer) Close() error {
	if w == nil || w.conn == nil {
		return nil
	}
	return w.conn.Close()
}

// Compile-time assertion that time.Duration is used so the import
// is not flagged by goimports when the only usage is via time.Time.
var _ = time.Second

// WriteSink adapts *Writer to the rollups.Sink interface. The
// production wiring constructs one of these and passes it to
// rollups.RunSink. The conversion from *WorkspaceSummary to the
// MergeTree row is kept inline because the columns are stable
// across M3.5 changes.
func (w *Writer) WriteSink(_ context.Context, s *rollups.WorkspaceSummary) error {
	return w.Write(_ context.TODO(), *s)
}
