package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/models"
)

// SQLite persistence for audit.

func (s *SQLite) AppendAudit(ctx context.Context, entry *models.AuditEntry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return errf.Errorf("marshal audit details: %w", err)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	entry.Timestamp = entry.Timestamp.UTC()
	// PERF-AUDIT-2: invalidate the verify cache so the next
	// VerifyAuditChain walks the full chain including this row.
	// Without this, a write-then-verify pair returns a stale
	// "ok" verdict that omits the just-appended entry.
	s.auditVerifyCache.Store(nil)
	timestampStr := entry.Timestamp.Format(time.RFC3339Nano)
	resourceKind, resourceID := splitAuditResource(entry.Resource)

	// Serialise the chain-link read + insert against any other
	// AppendAudit in this process. SQLite's serialisable
	// transaction below is the actual cross-process ordering
	// primitive; the cache mutex is the in-process companion
	// that keeps the cached (rowid, hash) pair consistent.
	s.auditTail.mu.Lock()
	defer s.auditTail.mu.Unlock()

	// CRIT-4 / DEF-14 sub-finding: bound the CAS retry loop.
	// Without this, a sustained contention pattern (writers that
	// always win the race) loops forever and the goroutine never
	// returns. Bounded to appendAuditMaxAttempts; the request's
	// context cancellation still aborts early via ctx.Err() above.
	const appendAuditMaxAttempts = 32
	for attempt := 0; attempt < appendAuditMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return errf.Errorf("append audit: %w", err)
		}

		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return errf.Errorf("begin audit tx: %w", err)
		}

		prevRowID, prevHash, err := s.tailHashLocked(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		entry.PreviousHash = prevHash
		entry.EntryHash = computeAuditHash(entry, string(details), timestampStr)

		insertRes, err := tx.ExecContext(ctx,
			`INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.ID, entry.UserID, entry.Action, entry.Resource,
			string(details), entry.Timestamp, entry.PreviousHash, entry.EntryHash, timestampStr,
			resourceKind, resourceID,
		)
		if err != nil {
			_ = tx.Rollback()
			return errf.Errorf("insert audit: %w", err)
		}
		rowID, err := insertRes.LastInsertId()
		if err != nil {
			_ = tx.Rollback()
			return errf.Errorf("last insert id: %w", err)
		}

		var stateRes sql.Result
		if prevRowID == 0 {
			stateRes, err = tx.ExecContext(ctx,
				`INSERT INTO audit_chain_state (id, last_hash, last_rowid)
				 VALUES (0, ?, ?)
				 ON CONFLICT(id) DO NOTHING`,
				entry.EntryHash, rowID,
			)
		} else {
			stateRes, err = tx.ExecContext(ctx,
				`UPDATE audit_chain_state
				 SET last_hash = ?, last_rowid = ?, updated_by_app = 1
				 WHERE id = 0 AND last_rowid = ? AND last_hash = ?`,
				entry.EntryHash, rowID, prevRowID, prevHash,
			)
		}
		if err != nil {
			_ = tx.Rollback()
			return errf.Errorf("update audit chain state: %w", err)
		}
		affected, err := stateRes.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return errf.Errorf("audit chain state rows affected: %w", err)
		}
		if affected == 0 {
			rollbackErr := tx.Rollback()
			s.auditTail.hash = ""
			s.auditTail.rowid.Store(0)
			if rollbackErr != nil {
				return errf.Errorf("rollback stale audit append: %w", rollbackErr)
			}
			continue
		}
		if err := tx.Commit(); err != nil {
			return errf.Errorf("commit audit: %w", err)
		}
		s.auditTail.hash = entry.EntryHash
		// #nosec G115 -- rowid is monotonically increasing from a
		// SQLite auto-increment primary key; wrap-around to negative
		// values would require >MaxInt64 inserts, which is not a
		// realistic production scenario.
		s.auditTail.rowid.Store(uint64(rowID))
		return nil
	}
	return errf.Errorf("append audit: CAS retry exhausted after %d attempts", appendAuditMaxAttempts)
}

// tailHashLocked returns the previous_hash for the next audit
// entry, populating the cache from audit_chain_state on first
// use. The caller must hold s.auditTail.mu.
//
// Read-through: the first AppendAudit (or any caller that bypassed
// the cache) initialises from the DB; subsequent calls take the
// fast path. The first query always re-reads the chain state
// row so a writer that bypassed AppendAudit (e.g. an admin SQL
// fix) is still chained correctly — the cached rowid is treated
// as a hint, not as the source of truth.
//
// The fast-path check happens UNDER the mutex so two concurrent
// first-time AppendAudits cannot both observe rowid=0 and chain
// against an empty previous_hash. atomic.Uint64 is used so the
// cross-package callers (e.g. diagnostics) can read the published
// rowid cheaply without taking the cache mutex.

func (s *SQLite) VerifyAuditChain(ctx context.Context) (*AuditVerifyResult, error) {
	// PERF-AUDIT-1: read the cached checkpoint. A fresh snapshot
	// (atomic.Pointer.Load) gives a consistent (rowid, hash) pair.
	cached := s.auditVerifyCache.Load()
	res, err := verifyAuditChainOnDB(ctx, s.db, cached)
	if err != nil {
		return nil, err
	}
	if res.Ok && res.LastRowID > 0 {
		s.auditVerifyCache.Store(&auditVerifyEntry{
			rowid: res.LastRowID,
			hash:  res.LastHash,
		})
	}
	return res, nil
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
		return nil, errf.Errorf("list audit: %w", err)
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

func (s *SQLite) ExportAudit(ctx context.Context, filter *models.AuditFilter) ([]*models.AuditEntry, error) {
	exportFilter := *filter
	exportFilter.Limit = 0
	exportFilter.Offset = 0
	return s.ListAudit(ctx, &exportFilter)
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *SQLite) tailHashLocked(ctx context.Context, tx *sql.Tx) (rowID int64, hash string, err error) {
	if cached := s.auditTail.rowid.Load(); cached != 0 && s.auditTail.hash != "" {
		// #nosec G115 -- same rationale as the Store calls below.
		return int64(cached), s.auditTail.hash, nil
	}
	queryErr := tx.QueryRowContext(ctx,
		`SELECT last_hash, last_rowid FROM audit_chain_state WHERE id = 0`,
	).Scan(&hash, &rowID)
	if queryErr != nil {
		if !errors.Is(queryErr, sql.ErrNoRows) {
			return 0, "", errf.Errorf("fetch previous audit hash: %w", queryErr)
		}
	}
	if hash != "" {
		s.auditTail.hash = hash
	}
	if rowID != 0 {
		// #nosec G115 -- see line 126.
		s.auditTail.rowid.Store(uint64(rowID))
	}
	return rowID, hash, nil
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

func VerifyAuditChainOnDB(ctx context.Context, db *sql.DB) (*AuditVerifyResult, error) {
	return verifyAuditChainOnDB(ctx, db, nil)
}

// verifyAuditChainOnDB is the inner implementation. The cache
// parameter is the optional PERF-AUDIT-1 checkpoint: when set,
// the walk starts after the cached rowid and verifies the
// cached hash matches the row at that checkpoint before
// continuing. cache may be nil for callers that don't have it
// (RetentionManager does not).

func verifyAuditChainOnDB(ctx context.Context, db *sql.DB, cache *auditVerifyEntry) (*AuditVerifyResult, error) {
	const pageSize = 1000
	var prevHash string
	var lastRowID int64
	// PERF-AUDIT-1: seed the walk from the cached checkpoint.
	if cache != nil && cache.rowid > 0 {
		// Verify the cached rowid still maps to the cached hash.
		// If it does, the prefix is intact and we only walk new
		// rows. If it doesn't, the cache is stale (someone
		// tampered or appended out-of-band) — fall back to a
		// full walk.
		var cachedHash string
		if err := db.QueryRowContext(ctx,
			`SELECT entry_hash FROM audit_entries WHERE rowid = ?`,
			cache.rowid,
		).Scan(&cachedHash); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, errf.Errorf("audit cache lookup: %w", err)
			}
			// Cached rowid is gone; full walk.
		} else if cachedHash == cache.hash {
			prevHash = cache.hash
			lastRowID = cache.rowid
		} // else: cache hash mismatch -> full walk from rowid 0
	}
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
			return nil, errf.Errorf("audit chain state: %w", err)
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

func verifyAuditPageOnDB(ctx context.Context, db *sql.DB, prevHash string, afterRowID int64, limit int) auditPageResult {
	const q = `SELECT rowid, id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str
	           FROM audit_entries
	           WHERE rowid > ?
	           ORDER BY rowid ASC
	           LIMIT ?`
	rows, err := db.QueryContext(ctx, q, afterRowID, limit)
	if err != nil {
		return auditPageResult{err: errf.Errorf("audit chain page query: %w", err)}
	}
	defer func() { _ = rows.Close() }()
	var nextPrev string
	var lastRowID int64
	for rows.Next() {
		var rowID int64
		var id, userID, action, resource, detailsJSON, storedPrev, storedHash, timestampStr string
		var ts time.Time
		if err := rows.Scan(&rowID, &id, &userID, &action, &resource, &detailsJSON, &ts, &storedPrev, &storedHash, &timestampStr); err != nil {
			return auditPageResult{err: errf.Errorf("audit chain scan: %w", err)}
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

func scanAuditRow(rows *sql.Rows) (*models.AuditEntry, error) {
	var e models.AuditEntry
	var details, prevHash, entryHash string
	err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Resource, &details, &e.Timestamp, &prevHash, &entryHash)
	if err != nil {
		return nil, errf.Errorf("scan audit entry: %w", err)
	}
	if err := json.Unmarshal([]byte(details), &e.Details); err != nil {
		slog.Error("failed to unmarshal audit details", "err", err, "id", e.ID)
	}
	e.PreviousHash = prevHash
	e.EntryHash = entryHash
	return &e, nil
}
