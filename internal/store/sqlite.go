// Package store provides the repository interface for data access.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/sachncs/promptsheon/internal/models"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a requested resource is not found.
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

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

// SQLite implements Repository backed by a SQLite database.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens or creates a SQLite database at dbPath and runs migrations.
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

	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error {
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

func (s *SQLite) AppendAudit(ctx context.Context, entry *models.AuditEntry) error {
	// The previous auditMu serialised every audit write through
	// Go-land, which defeated the 2-worker pool. The serialisable
	// SQLite transaction below is the actual ordering primitive;
	// SQLite serialises writers at the file level.
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	entry.Timestamp = entry.Timestamp.UTC()
	timestampStr := entry.Timestamp.Format(time.RFC3339Nano)

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin audit tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var prevHash string
	err = tx.QueryRowContext(ctx,
		`SELECT last_hash FROM audit_chain_state WHERE id = 0`,
	).Scan(&prevHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("fetch previous audit hash: %w", err)
	}

	entry.PreviousHash = prevHash
	entry.EntryHash = computeAuditHash(entry, string(details), timestampStr)

	// Split the resource string ("workspace:abc") into kind + id
	// for the structural query path (migration 048a). The legacy
	// `resource` column is preserved for backward compatibility;
	// the new columns are not part of the audit hash (the chain
	// format is unchanged).
	resourceKind, resourceID := splitAuditResource(entry.Resource)

	insertRes, err := tx.ExecContext(ctx,
		`INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Resource,
		string(details), entry.Timestamp, entry.PreviousHash, entry.EntryHash, timestampStr,
		resourceKind, resourceID,
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	rowID, err := insertRes.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	if _, e := tx.ExecContext(ctx,
		`INSERT INTO audit_chain_state (id, last_hash, last_rowid)
		 VALUES (0, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET last_hash = excluded.last_hash, last_rowid = excluded.last_rowid`,
		entry.EntryHash, rowID,
	); e != nil {
		return fmt.Errorf("update audit chain state: %w", e)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit audit: %w", err)
	}
	return nil
}

func computeAuditHash(e *models.AuditEntry, detailsJSON, timestampStr string) string {
	h := sha256.New()
	h.Write([]byte(e.ID))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.UserID))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.Action))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.Resource))
	h.Write([]byte{0x1f})
	h.Write([]byte(detailsJSON))
	h.Write([]byte{0x1f})
	h.Write([]byte(timestampStr))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.PreviousHash))
	return hex.EncodeToString(h.Sum(nil))
}

// AuditVerifyResult is the structured outcome of VerifyAuditChain
// (OBS-AUDIT-3). A UI can show the chain status without having
// to re-walk the chain itself.
//
// Ok is true only when the chain walk completed without an
// internal error AND the audit_chain_state rowid+hash cross-
// check matched the walked rowid+hash.
//
// TailMismatch is true when the walk completed but the
// audit_chain_state row points to a rowid or hash that the
// walk did not reach. This is the canonical tamper signal:
// a row was deleted out from under the chain state pointer.
//
// LastRowID / LastHash are the rowid and entry_hash of the
// last walked row. They match audit_chain_state.last_rowid /
// audit_chain_state.last_hash when Ok is true.
//
// Reason is a human-readable summary suitable for the audit
// log / SSE stream. It is non-empty whenever Ok is false.
type AuditVerifyResult struct {
	Ok            bool
	TailMismatch  bool
	LastRowID     int64
	LastHash      string
	Reason        string
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
func VerifyAuditChainOnDB(ctx context.Context, db *sql.DB) (*AuditVerifyResult, error) {
	const pageSize = 1000
	var prevHash string
	var lastRowID int64
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		res := verifyAuditPageOnDB(ctx, db, prevHash, lastRowID, pageSize)
		if res.err != nil {
			return nil, res.err
		}
		if !res.ok {
			return &AuditVerifyResult{Ok: false, Reason: res.reason}, nil
		}
		if res.lastRowID == 0 {
			break
		}
		prevHash = res.nextPrevHash
		lastRowID = res.lastRowID
	}

	// BUG-3 / SEC-CHAIN-1: cross-check against audit_chain_state.
	var stateLastRowID int64
	var stateLastHash string
	if err := db.QueryRowContext(ctx,
		`SELECT last_rowid, last_hash FROM audit_chain_state LIMIT 1`).Scan(&stateLastRowID, &stateLastHash); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("audit chain state: %w", err)
		}
	}
	if stateLastRowID != 0 && lastRowID != stateLastRowID {
		return &AuditVerifyResult{
			Ok:           false,
			TailMismatch: true,
			LastRowID:    lastRowID,
			LastHash:     prevHash,
			Reason:       fmt.Sprintf("audit chain tail mismatch: walked=%d, state=%d", lastRowID, stateLastRowID),
		}, nil
	}
	if stateLastHash != "" && prevHash != stateLastHash {
		return &AuditVerifyResult{
			Ok:           false,
			TailMismatch: true,
			LastRowID:    lastRowID,
			LastHash:     prevHash,
			Reason:       fmt.Sprintf("audit chain tail hash mismatch: walked=%s, state=%s", prevHash, stateLastHash),
		}, nil
	}
	return &AuditVerifyResult{
		Ok:        true,
		LastRowID: lastRowID,
		LastHash:  prevHash,
	}, nil
}

func (s *SQLite) VerifyAuditChain(ctx context.Context) (*AuditVerifyResult, error) {
	return VerifyAuditChainOnDB(ctx, s.db)
}

func verifyAuditPageOnDB(ctx context.Context, db *sql.DB, prevHash string, afterRowID int64, limit int) auditPageResult {
	const q = `SELECT rowid, id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str
	           FROM audit_entries
	           WHERE rowid > ?
	           ORDER BY rowid ASC
	           LIMIT ?`
	rows, err := db.QueryContext(ctx, q, afterRowID, limit)
	if err != nil {
		return auditPageResult{err: fmt.Errorf("audit chain page query: %w", err)}
	}
	defer func() { _ = rows.Close() }()
	var nextPrev string
	var lastRowID int64
	for rows.Next() {
		var rowID int64
		var id, userID, action, resource, detailsJSON, storedPrev, storedHash, timestampStr string
		var ts time.Time
		if err := rows.Scan(&rowID, &id, &userID, &action, &resource, &detailsJSON, &ts, &storedPrev, &storedHash, &timestampStr); err != nil {
			return auditPageResult{err: fmt.Errorf("audit chain scan: %w", err)}
		}
		if storedPrev != prevHash {
			return auditPageResult{
				ok:     false,
				reason: fmt.Sprintf("chain break at entry %s: expected prev_hash %q, got %q", id, prevHash, storedPrev),
			}
		}
		if timestampStr == "" {
			timestampStr = ts.UTC().Format(time.RFC3339Nano)
		}
		e := &models.AuditEntry{ID: id, UserID: userID, Action: action, Resource: resource, PreviousHash: storedPrev, Timestamp: ts}
		expected := computeAuditHash(e, detailsJSON, timestampStr)
		if expected != storedHash {
			return auditPageResult{
				ok:     false,
				reason: fmt.Sprintf("tampered entry %s: expected hash %q, got %q", id, expected, storedHash),
			}
		}
		prevHash = storedHash
		nextPrev = storedHash
		lastRowID = rowID
	}
	if err := rows.Err(); err != nil {
		return auditPageResult{err: err}
	}
	return auditPageResult{nextPrevHash: nextPrev, ok: true, lastRowID: lastRowID}
}

// splitAuditResource splits "kind:id" into (kind, id). Inputs
// without a colon return ("", input) so the structural columns
// are simply empty rather than wrong.
func splitAuditResource(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

func (s *SQLite) ListAudit(ctx context.Context, filter *models.AuditFilter) ([]*models.AuditEntry, error) {
	query := "SELECT id, user_id, action, resource, details, timestamp, previous_hash, entry_hash FROM audit_entries WHERE 1=1"
	args := []any{}

	if filter.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, filter.UserID)
	}
	if filter.ResourceKind != "" && filter.ResourceID != "" {
		// DB-8b: when the new structural columns are supplied,
		// use them in preference to the legacy "kind:id" string
		// in the `resource` column.
		query += " AND resource_kind = ? AND resource_id = ?"
		args = append(args, filter.ResourceKind, filter.ResourceID)
	} else if filter.Resource != "" {
		query += " AND resource = ?"
		args = append(args, filter.Resource)
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.Since != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.Until)
	}

	query += " ORDER BY timestamp DESC"

	// SQLite requires LIMIT before OFFSET and rejects an OFFSET
	// clause without a LIMIT. Use LIMIT -1 (no row cap) when the
	// caller asked for offset-only pagination.
	limit := filter.Limit
	if limit < 0 {
		limit = 0
	}
	if filter.Offset > 0 && limit == 0 {
		limit = -1
		query += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	} else {
		if limit > 0 {
			query += " LIMIT ?"
			args = append(args, limit)
		}
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*models.AuditEntry
	for rows.Next() {
		e, err := scanAuditRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func scanAuditRow(rows *sql.Rows) (*models.AuditEntry, error) {
	var e models.AuditEntry
	var details, prevHash, entryHash string
	err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Resource, &details, &e.Timestamp, &prevHash, &entryHash)
	if err != nil {
		return nil, fmt.Errorf("scan audit entry: %w", err)
	}
	if err := json.Unmarshal([]byte(details), &e.Details); err != nil {
		slog.Error("failed to unmarshal audit details", "err", err, "id", e.ID)
	}
	e.PreviousHash = prevHash
	e.EntryHash = entryHash
	return &e, nil
}

func (s *SQLite) ExportAudit(ctx context.Context, filter *models.AuditFilter) ([]*models.AuditEntry, error) {
	exportFilter := *filter
	exportFilter.Limit = 0
	exportFilter.Offset = 0
	return s.ListAudit(ctx, &exportFilter)
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *SQLite) CreateUser(ctx context.Context, u *models.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// BootstrapAdmin atomically inserts the first admin user and
// returns ErrConflict if any non-system user row already
// exists. The system user 'api' (seeded by migration 057) is
// ignored so the bootstrap endpoint stays available even when
// the audit FK has been satisfied.
//
// BootstrapAdmin is the single-caller bootstrap. SEC-5a: 100
// concurrent POST /api/v1/setup calls must produce exactly one
// admin key, with the rest seeing ErrConflict. The race-free
// path is INSERT ... ON CONFLICT (email) DO NOTHING: SQLite
// resolves the conflict at write time, so even under a
// DEFERRED transaction the second writer's INSERT silently
// drops and RowsAffected returns 0. We then check the rows-
// affected count and return ErrConflict for the loser.
//
// The previous implementation used a SELECT COUNT(*) then a
// plain INSERT; under DEFERRED locking both writers could read
// the same empty count and both insert successfully, minting
// two admin keys.
func (s *SQLite) BootstrapAdmin(ctx context.Context, u *models.User, key *models.APIKey) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("bootstrap begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (email) DO NOTHING`,
		u.ID, u.Email, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("bootstrap insert user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("bootstrap rows affected: %w", err)
	}
	if n == 0 {
		// Another caller won the race. Roll back the key insert
		// (we never get there) and surface a typed conflict.
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.UserID, key.Name, key.KeyHash, key.KeyPrefix, key.Role, key.CreatedAt,
	); err != nil {
		return fmt.Errorf("bootstrap insert key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("bootstrap commit: %w", err)
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
		return nil, fmt.Errorf("list users: %w", err)
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
		return fmt.Errorf("update user begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Look up the existing role so we only revoke API keys when
	// the role actually changed. Without this, every PUT on the
	// user record would invalidate live keys.
	var oldRole string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, u.ID).Scan(&oldRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user not found: %s", u.ID)
		}
		return fmt.Errorf("update user lookup: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE users SET email=?, name=?, role=?, updated_at=? WHERE id=?`,
		u.Email, u.Name, u.Role, u.UpdatedAt, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %s", u.ID)
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
			return fmt.Errorf("revoke stale api keys: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("update user commit: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete user begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET revoked = 1 WHERE user_id = ? AND revoked = 0`, id); err != nil {
		return fmt.Errorf("revoke keys on delete: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %s", id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete user commit: %w", err)
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func scanUserRow(rows *sql.Rows) (*models.User, error) {
	return scanUser(rows)
}

// --- API Keys ---

func (s *SQLite) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	const q = `INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		key.ID, key.UserID, key.Name, key.KeyHash, key.KeyPrefix,
		key.Role, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLite) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE key_hash = ? AND revoked = 0`
	var k models.APIKey
	err := s.db.QueryRowContext(ctx, q, keyHash).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return &k, nil
}

func (s *SQLite) GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE id = ?`
	var k models.APIKey
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	return &k, nil
}

func (s *SQLite) DeleteAPIKey(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET revoked = 1 WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *SQLite) ListAPIKeysByUser(ctx context.Context, userID string) ([]*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (s *SQLite) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET last_used = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Provider Keys
// ---------------------------------------------------------------------------

func (s *SQLite) SaveProviderKey(ctx context.Context, pk *models.ProviderKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO provider_keys (id, provider_name, key_name, encrypted_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET encrypted_key=excluded.encrypted_key, updated_at=excluded.updated_at`,
		pk.ID, pk.ProviderName, pk.KeyName, pk.EncryptedKey, pk.CreatedAt, pk.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save provider key: %w", err)
	}
	return nil
}

func (s *SQLite) GetProviderKey(ctx context.Context, id string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE id = ?`, id,
	)
	return scanProviderKey(row)
}

func (s *SQLite) GetProviderKeyByName(ctx context.Context, providerName, keyName string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE provider_name = ? AND key_name = ?`, providerName, keyName,
	)
	return scanProviderKey(row)
}

func (s *SQLite) DeleteProviderKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM provider_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete provider key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider key not found: %s", id)
	}
	return nil
}

func (s *SQLite) ListProviderKeys(ctx context.Context) ([]*models.ProviderKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*models.ProviderKey
	for rows.Next() {
		k, err := scanProviderKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func scanProviderKey(row scannable) (*models.ProviderKey, error) {
	var pk models.ProviderKey
	err := row.Scan(&pk.ID, &pk.ProviderName, &pk.KeyName, &pk.EncryptedKey, &pk.CreatedAt, &pk.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan provider key: %w", err)
	}
	return &pk, nil
}

func scanProviderKeyRow(rows *sql.Rows) (*models.ProviderKey, error) {
	return scanProviderKey(rows)
}

// Alert Rules

func (s *SQLite) SaveAlertRule(ctx context.Context, r *models.AlertRuleRecord) error {
	configJSON, err := marshalOrErr(r.Config)
	if err != nil {
		return fmt.Errorf("marshal alert rule config: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO alert_rules (id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			severity = excluded.severity,
			enabled = excluded.enabled,
			threshold = excluded.threshold,
			duration = excluded.duration,
			window = excluded.window,
			config = excluded.config,
			updated_at = excluded.updated_at`,
		r.ID, r.Name, r.Type, r.Severity, r.Enabled, r.Threshold, r.Duration, r.Window, configJSON, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save alert rule: %w", err)
	}
	return nil
}

func (s *SQLite) GetAlertRule(ctx context.Context, id string) (*models.AlertRuleRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at
		FROM alert_rules WHERE id = ?`, id)
	return scanAlertRule(row)
}

func (s *SQLite) DeleteAlertRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}

func (s *SQLite) ListAlertRules(ctx context.Context) ([]*models.AlertRuleRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rules []*models.AlertRuleRecord
	for rows.Next() {
		r, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func scanAlertRule(row scannable) (*models.AlertRuleRecord, error) {
	var r models.AlertRuleRecord
	var configJSON string
	err := row.Scan(
		&r.ID, &r.Name, &r.Type, &r.Severity, &r.Enabled, &r.Threshold, &r.Duration, &r.Window,
		&configJSON, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan alert rule: %w", err)
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &r.Config); err != nil {
			slog.Error("failed to unmarshal alert rule config", "err", err, "id", r.ID)
		}
	}
	return &r, nil
}

// Alerts

func (s *SQLite) SaveAlert(ctx context.Context, a *models.AlertRecord) error {
	detailsJSON, err := marshalOrErr(a.Details)
	if err != nil {
		return fmt.Errorf("marshal alert details: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO alerts (id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RuleID, a.RuleName, a.Severity, a.Status, a.Message, detailsJSON, a.TriggeredAt, a.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("save alert: %w", err)
	}
	return nil
}

func (s *SQLite) GetAlert(ctx context.Context, id string) (*models.AlertRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at
		FROM alerts WHERE id = ?`, id)
	return scanAlert(row)
}

func (s *SQLite) UpdateAlert(ctx context.Context, a *models.AlertRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE alerts SET status=?, resolved_at=? WHERE id=?`,
		a.Status, a.ResolvedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alert not found: %s", a.ID)
	}
	return nil
}

func (s *SQLite) ListAlerts(ctx context.Context, status string) ([]*models.AlertRecord, error) {
	query := `SELECT id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at FROM alerts`
	var args []any
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY triggered_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var alerts []*models.AlertRecord
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func scanAlert(row scannable) (*models.AlertRecord, error) {
	var a models.AlertRecord
	var detailsJSON string
	var resolvedAt *time.Time
	err := row.Scan(
		&a.ID, &a.RuleID, &a.RuleName, &a.Severity, &a.Status, &a.Message,
		&detailsJSON, &a.TriggeredAt, &resolvedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan alert: %w", err)
	}
	if detailsJSON != "" {
		if err := json.Unmarshal([]byte(detailsJSON), &a.Details); err != nil {
			slog.Error("failed to unmarshal alert details", "err", err, "id", a.ID)
		}
	}
	a.ResolvedAt = resolvedAt
	return &a, nil
}

// Notification Groups

func (s *SQLite) SaveNotificationGroup(ctx context.Context, g *models.NotificationGroupRecord) error {
	channelsJSON, err := marshalOrErr(g.Channels)
	if err != nil {
		return fmt.Errorf("marshal notification channels: %w", err)
	}
	// INSERT ... ON CONFLICT DO UPDATE preserves any child rows in
	// alert_rule_notification_groups that reference this group's
	// primary key. INSERT OR REPLACE deletes + reinserts and would
	// cascade the M2M link into oblivion.
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO notification_groups (id, name, channels)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, channels=excluded.channels`,
		g.ID, g.Name, channelsJSON,
	)
	if err != nil {
		return fmt.Errorf("save notification group: %w", err)
	}
	return nil
}

func (s *SQLite) GetNotificationGroup(ctx context.Context, id string) (*models.NotificationGroupRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, channels FROM notification_groups WHERE id = ?`, id)
	return scanNotificationGroup(row)
}

func (s *SQLite) DeleteNotificationGroup(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete notification group: %w", err)
	}
	return nil
}

func (s *SQLite) ListNotificationGroups(ctx context.Context) ([]*models.NotificationGroupRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, channels FROM notification_groups ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list notification groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var groups []*models.NotificationGroupRecord
	for rows.Next() {
		g, err := scanNotificationGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetChannelsForAlertRule returns the union of channels across
// all notification groups wired to the rule. Returns nil if the
// rule has no M2M rows.
//
// DB-10a: the channels column is JSON-encoded (e.g.
// '["webhook","log"]'). The query flattens the JSON array into
// rows via SQLite's json_each so each channel arrives as its
// own row, dedup happens at the SQL level (DISTINCT), and the
// alerting manager receives a stable sorted list. No Go-side
// JSON unmarshal of the channel blob.
func (s *SQLite) GetChannelsForAlertRule(ctx context.Context, ruleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT json_each.value
		   FROM alert_rule_notification_groups m2m
		   JOIN notification_groups ng ON ng.id = m2m.notification_group_id
		   JOIN json_each(ng.channels)
		  WHERE m2m.alert_rule_id = ?
		  ORDER BY json_each.value`,
		ruleID)
	if err != nil {
		return nil, fmt.Errorf("channels for alert rule: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var channels []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan channels: %w", err)
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return channels, nil
}

func scanNotificationGroup(row scannable) (*models.NotificationGroupRecord, error) {
	var g models.NotificationGroupRecord
	var channelsJSON string
	err := row.Scan(&g.ID, &g.Name, &channelsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan notification group: %w", err)
	}
	if channelsJSON != "" {
		if err := json.Unmarshal([]byte(channelsJSON), &g.Channels); err != nil {
			slog.Error("failed to unmarshal notification channels", "err", err, "id", g.ID)
		}
	}
	return &g, nil
}

// ---------------------------------------------------------------------------
// Webhook Endpoints
// ---------------------------------------------------------------------------

func (s *SQLite) SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error {
	events := strings.Join(ep.Events, ",")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_endpoints (id, url, secret_ciphertext, events, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			url = excluded.url,
			secret_ciphertext = excluded.secret_ciphertext,
			events = excluded.events,
			active = excluded.active`,
		ep.ID, ep.URL, ep.SecretCiphertext, events, ep.Active, ep.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, url, secret_ciphertext, events, active, created_at FROM webhook_endpoints WHERE id = ?`, id)
	ep, err := scanWebhookEndpoint(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return ep, err
}

func (s *SQLite) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhook_endpoints WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret_ciphertext, events, active, created_at FROM webhook_endpoints ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var eps []*models.WebhookEndpointRecord
	for rows.Next() {
		ep, err := scanWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}

func scanWebhookEndpoint(row scannable) (*models.WebhookEndpointRecord, error) {
	var ep models.WebhookEndpointRecord
	var events string
	err := row.Scan(&ep.ID, &ep.URL, &ep.SecretCiphertext, &events, &ep.Active, &ep.CreatedAt)
	if err != nil {
		return nil, err
	}
	if events != "" {
		ep.Events = strings.Split(events, ",")
	}
	return &ep, nil
}

// GetVaultState returns the singleton vault_state row, or nil if
// no wrapped data key has been persisted yet. SEC-10a.
func (s *SQLite) GetVaultState(ctx context.Context) (*models.VaultState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT kms_key_id, wrapped_data_key, created_at, updated_at
		   FROM vault_state WHERE id = 0`)
	var vs models.VaultState
	err := row.Scan(&vs.KMSKeyID, &vs.WrappedDataKey, &vs.CreatedAt, &vs.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &vs, nil
}

// SaveVaultState persists the singleton vault_state row. Creates
// the row on first use; subsequent calls update it. SEC-10a.
func (s *SQLite) SaveVaultState(ctx context.Context, vs *models.VaultState) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vault_state (id, kms_key_id, wrapped_data_key, created_at, updated_at)
		VALUES (0, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kms_key_id = excluded.kms_key_id,
			wrapped_data_key = excluded.wrapped_data_key,
			updated_at = excluded.updated_at`,
		vs.KMSKeyID, vs.WrappedDataKey, now, now,
	)
	if err != nil {
		return fmt.Errorf("save vault state: %w", err)
	}
	vs.CreatedAt = now
	vs.UpdatedAt = now
	return nil
}

// GetWSNextID returns the persisted SSE hub next-id, or 0 if
// none has been stored yet. OBS-LOG-3.
func (s *SQLite) GetWSNextID(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT next_id FROM ws_state WHERE id = 0`,
	).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get ws next id: %w", err)
	}
	return n, nil
}

// SetWSNextID persists the SSE hub's next-id so a restart can
// continue from the same point. OBS-LOG-3.
func (s *SQLite) SetWSNextID(ctx context.Context, n int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ws_state (id, next_id, updated_at)
		VALUES (0, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			next_id = excluded.next_id,
			updated_at = excluded.updated_at`,
		n,
	)
	if err != nil {
		return fmt.Errorf("set ws next id: %w", err)
	}
	return nil
}

// GetEnforcerBudget returns the persisted budget payload for a
// workspace, or nil if none. OBS-13.
func (s *SQLite) GetEnforcerBudget(ctx context.Context, workspaceID string) ([]byte, error) {
	return s.getEnforcerPayload(ctx, workspaceID, "budget")
}

// SetEnforcerBudget persists the budget payload for a workspace.
// OBS-13.
func (s *SQLite) SetEnforcerBudget(ctx context.Context, workspaceID string, payload []byte) error {
	return s.setEnforcerPayload(ctx, workspaceID, "budget", payload)
}

// GetEnforcerQuota returns the persisted quota payload for a
// workspace, or nil if none. OBS-13.
func (s *SQLite) GetEnforcerQuota(ctx context.Context, workspaceID string) ([]byte, error) {
	return s.getEnforcerPayload(ctx, workspaceID, "quota")
}

// SetEnforcerQuota persists the quota payload for a workspace.
// OBS-13.
func (s *SQLite) SetEnforcerQuota(ctx context.Context, workspaceID string, payload []byte) error {
	return s.setEnforcerPayload(ctx, workspaceID, "quota", payload)
}

func (s *SQLite) getEnforcerPayload(ctx context.Context, workspaceID, kind string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM enforcer_state WHERE workspace_id = ? AND kind = ?`,
		workspaceID, kind,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enforcer %s for %s: %w", kind, workspaceID, err)
	}
	return payload, nil
}

func (s *SQLite) setEnforcerPayload(ctx context.Context, workspaceID, kind string, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO enforcer_state (workspace_id, kind, payload, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(workspace_id, kind) DO UPDATE SET
			payload = excluded.payload,
			updated_at = excluded.updated_at`,
		workspaceID, kind, payload,
	)
	if err != nil {
		return fmt.Errorf("set enforcer %s for %s: %w", kind, workspaceID, err)
	}
	return nil
}
