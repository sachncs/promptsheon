package store

import (
	"github.com/sachncs/promptsheon/errf"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite struct, core connection lifecycle, and shared helpers.
// Per-entity methods are split into sqlite_<entity>.go files.

//go:embed migrations/*.sql
var migrationsFS embed.FS

type SQLite struct {
	db *sql.DB

	// PERF-DB-1: prepared statements for hot read paths. These
	// are prepared once at construction and reused across every
	// call, eliminating the per-call SQL parse + plan cost on
	// the dashboard hot path. They are closed on Close().
	stmtGetRelease       *sql.Stmt
	stmtGetCapability    *sql.Stmt
	stmtGetAPIKeyByHash  *sql.Stmt
	stmtListExecutionsCV *sql.Stmt

	// PERF-AUDIT-1: cache the last verified (rowid, hash) pair
	// so repeat VerifyAuditChain calls only walk the new rows.
	// The cache is invalidated on every AppendAudit (the fresh
	// rowid is below the cached checkpoint, so the walk still
	// covers it). On the first call we walk the full chain.
	auditVerifyCache atomic.Pointer[auditVerifyEntry]

	// auditTail caches the (last_rowid, last_hash) pair that
	// AppendAudit must chain against. Reading from the cache
	// removes one SELECT per append; the cache is initialised
	// read-through on first use and updated after every
	// successful append. Concurrency:
	//
	//   - auditTail.rowid is an atomic.Uint64 used as both the
	//     initialised sentinel (0 = unknown) and the published
	//     rowid value.
	//   - auditTail.hash is protected by auditTail.mu because
	//     the (rowid, hash) pair must be read/written together
	//     and Go strings cannot be assigned atomically.
	//
	// AppendAudit takes auditTail.mu for the duration of its
	// chain + insert, so two concurrent appends serialise the
	// cache mutation the same way SQLite serialises writers.
	auditTail struct {
		mu    sync.Mutex
		rowid atomic.Uint64
		hash  string
	}
}

// auditVerifyEntry is the cached (rowid, hash) pair from the
// last successful VerifyAuditChain call.
type auditVerifyEntry struct {
	rowid int64
	hash  string
}

// AuditVerifyResult reports the outcome of a chain-verification
// walk.
type AuditVerifyResult struct {
	Ok           bool
	TailMismatch bool
	LastRowID    int64
	LastHash     string
	Reason       string
}

type auditPageResult struct {
	nextPrevHash string
	ok           bool
	reason       string
	lastRowID    int64
	err          error
}

// scannable is the minimum surface a row source must expose for
// the per-entity scan helpers in this package.
type scannable interface {
	Scan(dest ...any) error
}

func marshalOrErr(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errf.Errorf("marshal json: %w", err)
	}
	return b, nil
}

func mustUnmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		// CRIT-1 / DEF-13: surface corruption to callers instead of
		// silently zeroing the struct. The previous behaviour masked
		// disk corruption, schema drift, and interrupted writes behind
		// a slog.Error log; upstream validation failures had no audit
		// trail pointing at the corrupted row.
		return errf.Errorf("unmarshal JSON: %w", err)
	}
	return nil
}

func NewSQLite(dbPath string) (*SQLite, error) {
	pragmas := "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
	var dsn string
	if dbPath == ":memory:" {
		dsn = "file::memory:?cache=shared&_pragma=journal_mode(MEMORY)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	} else {
		dsn = dbPath + "?" + pragmas
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errf.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := migrate(db, migrationsFS); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, errf.Errorf("migrate: %w (close: %v)", err, cerr)
		}
		return nil, errf.Errorf("migrate: %w", err)
	}

	s := &SQLite{db: db}
	// PERF-DB-1: prepare hot-path statements. The SQL is a static
	// literal, so a prepare error indicates the migration did not
	// create the expected schema. Surface it as a wrapped
	// initialization error so the daemon fails fast at startup
	// rather than silently falling back to non-prepared SQL on
	// every call.
	prep := func(query string) (*sql.Stmt, error) {
		stmt, err := db.Prepare(query)
		if err != nil {
			return nil, errf.Errorf("prepare statement: %w", err)
		}
		return stmt, nil
	}
	stmtGetRelease, err := prep(`SELECT id, capability_id, capability_version, manifest, environment, status,
		approved_by, superseded_by, replaces_release_id,
		created_at, created_by, activated_at, superseded_at,
		canary_percent
	 FROM releases WHERE id = ?`)
	if err != nil {
		return nil, err
	}
	s.stmtGetRelease = stmtGetRelease
	stmtGetCapability, err := prep(`SELECT id, project_id, name, description, created_at, updated_at,
	 self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec,
	 self_evolve_target_env, self_evolve_dataset_id
	 FROM capabilities WHERE id = ?`)
	if err != nil {
		return nil, err
	}
	s.stmtGetCapability = stmtGetCapability
	stmtGetAPIKeyByHash, err := prep(`SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
	 FROM api_keys WHERE key_hash = ? AND revoked = 0`)
	if err != nil {
		return nil, err
	}
	s.stmtGetAPIKeyByHash = stmtGetAPIKeyByHash
	stmtListExecutionsCV, err := prep(`SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
	 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
	 error, trace_id, environment FROM executions WHERE capability_version_id = ?
	 ORDER BY timestamp DESC LIMIT ?`)
	if err != nil {
		return nil, err
	}
	s.stmtListExecutionsCV = stmtListExecutionsCV

	return s, nil
}

func (s *SQLite) Close() error {
	// PERF-DB-1: close prepared statements before the DB.
	var closeErr error
	for _, stmt := range []*sql.Stmt{
		s.stmtGetRelease,
		s.stmtGetCapability,
		s.stmtGetAPIKeyByHash,
		s.stmtListExecutionsCV,
	} {
		if stmt == nil {
			continue
		}
		if err := stmt.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	if err := s.db.Close(); err != nil && closeErr == nil {
		closeErr = err
	}
	return closeErr
}

func (s *SQLite) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLite) DB() *sql.DB {
	return s.db
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------
