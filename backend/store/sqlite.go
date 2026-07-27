package store
import (
	"context"
	"database/sql"
	"embed"

	_ "modernc.org/sqlite"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

)

// SQLite struct, core connection lifecycle, and shared helpers.
// Per-entity methods are split into sqlite_<entity>.go files.

//go:embed migrations/*.sql
var migrationsFS embed.FS

// errs.ErrorStoreNotFound is returned when a requested resource is not found.


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

// NewSQLite opens or creates a SQLite database at dbPath and runs migrations.

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

// VerifyAuditChainOnDB runs the chain walk against an arbitrary
// *sql.DB. Used by RetentionManager to verify the chain before
// archiving audit rows. The function is package-level so the
// observability package can call it without importing the
// store's Repository surface.

type scannable interface {
	Scan(dest ...any) error
}


type scannableCRDT interface {
	Scan(dest ...any) error
}


func marshalOrErr(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return b, nil
}


func mustUnmarshal(data []byte, v any) {
	if len(data) == 0 {
		return
	}
	if err := json.Unmarshal(data, v); err != nil {
		slog.Error("failed to unmarshal JSON", "err", err)
	}
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
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := migrate(db, migrationsFS); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s := &SQLite{db: db}
	// PERF-DB-1: prepare hot-path statements. Failure here is a
	// programmer error (the SQL is a static literal) so we
	// panic if any statement cannot be prepared — the daemon
	// cannot start otherwise.
	if stmt, err := db.Prepare(`SELECT id, capability_id, capability_version, manifest, environment, status,
		approved_by, superseded_by, replaces_release_id,
		created_at, created_by, activated_at, superseded_at
	 FROM releases WHERE id = ?`); err == nil {
		s.stmtGetRelease = stmt
	}
	if stmt, err := db.Prepare(`SELECT id, project_id, name, description, created_at, updated_at,
	 self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec,
	 self_evolve_target_env, self_evolve_dataset_id
	 FROM capabilities WHERE id = ?`); err == nil {
		s.stmtGetCapability = stmt
	}
	if stmt, err := db.Prepare(`SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
	 FROM api_keys WHERE key_hash = ? AND revoked = 0`); err == nil {
		s.stmtGetAPIKeyByHash = stmt
	}
	if stmt, err := db.Prepare(`SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
	 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
	 error, trace_id, environment FROM executions WHERE capability_version_id = ?
	 ORDER BY timestamp DESC LIMIT ?`); err == nil {
		s.stmtListExecutionsCV = stmt
	}

	return s, nil
}


func (s *SQLite) Close() error {
	// PERF-DB-1: close prepared statements before the DB.
	for _, stmt := range []*sql.Stmt{
		s.stmtGetRelease,
		s.stmtGetCapability,
		s.stmtGetAPIKeyByHash,
		s.stmtListExecutionsCV,
	} {
		if stmt != nil {
			_ = stmt.Close()
		}
	}
	return s.db.Close()
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


