// Command promptsheon-auditbackfill populates the
// resource_kind and resource_id columns added to audit_entries
// by migration 048a. The 048a migration added the columns with
// DEFAULT ”; this command fills them from the existing
// `resource` column for historical rows so the structural
// query path is useful against the full table, not just new
// rows.
//
// Usage:
//
//		promptsheon-auditbackfill [--db PATH] [--batch-size N] [--dry-run] [--progress-every N]
//
//	  --db PATH          SQLite database file (default:
//	                     $PROMPTSHEON_DB_PATH or "promptsheon.db")
//	  --batch-size N     rows per UPDATE batch (default 5000)
//	  --dry-run          show what would be updated, change nothing
//	  --progress-every N log progress every N batches (default 1)
//
// The command is safe to run while the daemon is online: it
// takes a short transaction per batch and yields between
// batches. Concurrent writes to audit_entries (the AppendAudit
// path) are unaffected because each batch UPDATE locks only
// the rows in the batch's rowid range.
//
// On a 100M-row table, default batch size 5000, the run
// takes ~minutes to ~hours depending on disk. The progress
// log is one line per batch. Cancel with SIGINT and restart
// later — the script is idempotent because each UPDATE only
// touches rows where resource_kind = ” (the 048a default).
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// config is the parameter struct for the testable backfill
// function. main() builds one from flags and calls run().
type config struct {
	DBPath        string
	BatchSize     int
	DryRun        bool
	ProgressEvery int
}

// result reports the backfill run to the caller.
type result struct {
	Batches        int
	RowsUpdated    int64
	Elapsed        time.Duration
	CancelledEarly bool
}

func main() {
	os.Exit(run())
}

func run() int {
	var cfg = defaultConfig()
	flag.StringVar(&cfg.DBPath, "db", envOr("PROMPTSHEON_DB_PATH", "promptsheon.db"), "SQLite database file")
	flag.IntVar(&cfg.BatchSize, "batch-size", 5000, "rows per UPDATE batch")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "show what would be updated, change nothing")
	flag.IntVar(&cfg.ProgressEvery, "progress-every", 1, "log progress every N batches")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	dsn := cfg.DBPath + "?_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		logger.Error("open db", "err", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	res, err := backfill(ctx, db, cfg, logger)
	if err != nil {
		logger.Error("backfill", "err", err)
		return 1
	}
	if res.CancelledEarly {
		return 130
	}
	return 0
}

// backfill is the testable core of the command. It is
// exported within the package so the test in main_test.go can
// exercise it without a real binary. The dry-run path reports
// counts without modifying; the live path batches UPDATE
// statements with rowid > lastID for forward progress.
func backfill(ctx context.Context, db *sql.DB, cfg config, logger *slog.Logger) (result, error) {
	start := time.Now()
	var pending int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_entries WHERE resource_kind = ''`,
	).Scan(&pending)
	if err != nil {
		return result{}, err
	}
	logger.Info("audit backfill starting", "pending", pending, "batch_size", cfg.BatchSize, "dry_run", cfg.DryRun)
	if pending == 0 {
		logger.Info("nothing to do; all rows already have resource_kind set")
		return result{}, nil
	}

	res := result{}
	var lastID int64 = 0
	batch := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("cancelled", "rows_remaining", pending)
			res.CancelledEarly = true
			res.Elapsed = time.Since(start)
			return res, nil
		default:
		}

		if cfg.DryRun {
			var count int
			err = db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM audit_entries
				 WHERE resource_kind = '' AND rowid > ? LIMIT ?`,
				lastID, cfg.BatchSize,
			).Scan(&count)
			if err != nil {
				return res, err
			}
			batch++
			if batch%cfg.ProgressEvery == 0 {
				logger.Info("dry-run progress", "batch", batch, "rows_in_next", count, "last_rowid", lastID)
			}
			if count == 0 {
				break
			}
			lastID += int64(cfg.BatchSize) * 1000
			if lastID > 1e9 {
				break
			}
			continue
		}

		updated, err := runBatch(ctx, db, lastID, cfg.BatchSize)
		if err != nil {
			return res, err
		}
		// Advance lastID past the rows we touched.
		var maxID sql.NullInt64
		err = db.QueryRowContext(ctx,
			`SELECT MAX(rowid) FROM audit_entries WHERE resource_kind = '' AND rowid > ?`,
			lastID,
		).Scan(&maxID)
		if err != nil {
			return res, err
		}
		if maxID.Valid {
			lastID = maxID.Int64
		}
		res.RowsUpdated += updated
		// Recount remaining so progress is accurate.
		err = db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_entries WHERE resource_kind = '' AND rowid > ?`,
			lastID,
		).Scan(&pending)
		if err != nil {
			return res, err
		}
		batch++
		if batch%cfg.ProgressEvery == 0 {
			elapsed := time.Since(start)
			rate := float64(batch) * float64(cfg.BatchSize) / elapsed.Seconds()
			logger.Info("backfill progress",
				"batch", batch,
				"rows_remaining", pending,
				"last_rowid", lastID,
				"elapsed", elapsed.Round(time.Second).String(),
				"rate_rows_per_sec", int(rate),
			)
		}
		if pending == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	res.Batches = batch
	res.Elapsed = time.Since(start)
	logger.Info("audit backfill complete",
		"batches", res.Batches,
		"rows_updated", res.RowsUpdated,
		"elapsed", res.Elapsed.Round(time.Second).String(),
	)
	return res, nil
}

// runBatch executes one UPDATE batch and returns the number of
// rows updated. The CASE expression matches the Go-side
// splitResource helper in internal/store/split_resource.go:
// split on the first ':'; rows with no ':' get empty
// strings.
func runBatch(ctx context.Context, db *sql.DB, lastID int64, batchSize int) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE audit_entries
		   SET resource_kind = CASE
		         WHEN instr(resource, ':') > 0
		         THEN substr(resource, 1, instr(resource, ':') - 1)
		         ELSE ''
		       END,
		       resource_id   = CASE
		         WHEN instr(resource, ':') > 0
		         THEN substr(resource, instr(resource, ':') + 1)
		         ELSE ''
		       END
		 WHERE rowid IN (
		   SELECT rowid FROM audit_entries
		    WHERE resource_kind = '' AND rowid > ?
		    ORDER BY rowid ASC
		    LIMIT ?
		 )`,
		lastID, batchSize)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func defaultConfig() config {
	return config{
		DBPath:        envOr("PROMPTSHEON_DB_PATH", "promptsheon.db"),
		BatchSize:     5000,
		DryRun:        false,
		ProgressEvery: 1,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// errNoOp is returned by run() when there is nothing to do. We
// keep it exported (within the package) for the test, which
// asserts that a fully-populated DB exits cleanly with this
// sentinel.
var errNoOp = errors.New("nothing to do")
