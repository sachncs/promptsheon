// Package retention provides the periodic cleanup of expired
// traces and audit rows. It owns the policy, the ticker, and
// the chain-verification gate that prevents the sweep from
// archiving rows when the audit chain is broken.
package retention

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/store"
)

// Policy defines TTL for different log types.
type Policy struct {
	TraceTTL      time.Duration
	AuditTTL      time.Duration
	CheckInterval time.Duration
}

// DefaultPolicy returns sensible defaults.
func DefaultPolicy() Policy {
	return Policy{
		TraceTTL:      30 * 24 * time.Hour, // 30 days minimum
		AuditTTL:      90 * 24 * time.Hour, // 90 days
		CheckInterval: 1 * time.Hour,       // check every hour
	}
}

// LoadPolicyFromEnv loads retention policy from environment variables.
// Supported env vars: PROMPTSHEON_TRACE_TTL_DAYS, PROMPTSHEON_AUDIT_TTL_DAYS,
// PROMPTSHEON_RETENTION_CHECK_MINUTES.
func LoadPolicyFromEnv() Policy {
	p := DefaultPolicy()

	if v := os.Getenv("PROMPTSHEON_TRACE_TTL_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days >= 1 {
			p.TraceTTL = time.Duration(days) * 24 * time.Hour
		}
	}
	if v := os.Getenv("PROMPTSHEON_AUDIT_TTL_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days >= 1 {
			p.AuditTTL = time.Duration(days) * 24 * time.Hour
		}
	}
	if v := os.Getenv("PROMPTSHEON_RETENTION_CHECK_MINUTES"); v != "" {
		if mins, err := strconv.Atoi(v); err == nil && mins >= 1 {
			p.CheckInterval = time.Duration(mins) * time.Minute
		}
	}

	// Enforce minimum 30-day trace retention
	if p.TraceTTL < 30*24*time.Hour {
		p.TraceTTL = 30 * 24 * time.Hour
	}

	return p
}

// Manager handles periodic cleanup of expired data.
type Manager struct {
	db      *sql.DB
	policy  Policy
	logger  *slog.Logger
	lastErr error
}

// New creates a retention manager.
func New(db *sql.DB, policy Policy, logger *slog.Logger) *Manager {
	return &Manager{
		db:     db,
		policy: policy,
		logger: logger,
	}
}

// Start begins the periodic cleanup goroutine.
func (m *Manager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.policy.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.Enforce(ctx); err != nil {
					m.logger.Error("retention enforcement failed", "err", err)
				}
			}
		}
	}()
}

// ProtectedAuditActions is the documented invariant that the
// security review classified as "must never be lost". The table
// is exported so test fixtures and audit-archive tooling can
// read it.
//
// Retention never deletes audit rows from audit_entries (the
// chain walks rowid 1 forward by previous_hash). Removing rows
// in the middle is unrecoverable. Operators archive externally.
var ProtectedAuditActions = map[string]bool{
	"auth_failure":     true,
	"auto_approve":     true,
	"deploy":           true,
	"create":           true,
	"update":           true,
	"delete":           true,
	"restore":          true,
	"approve":          true,
	"reject":           true,
	"permission_grant": true,
	"key_mint":         true,
	"key_revoke":       true,
}

// Enforce deletes expired data based on the retention policy.
//
// OPS-3: every error path returns the wrapped error so callers
// can log + surface a metric. The previous version logged and
// returned nil, masking persistent SQLite errors as success.
//
// OBS-RET-1: audit rows are NOT deleted from audit_entries.
// The chain walks from rowid 1 forward and chains by
// previous_hash; deleting a row in the middle breaks
// verification. Instead, expired audit rows are copied into
// audit_archive (created by migration 011). The source row is
// preserved so the chain survives; operators archive externally
// and may then truncate the source table out of band.
//
// Returns the wrapped error from any failure.
func (m *Manager) Enforce(ctx context.Context) error {
	m.lastErr = nil
	var traceDeleted, auditArchived int

	if m.policy.TraceTTL > 0 {
		cutoff := time.Now().Add(-m.policy.TraceTTL)
		result, err := m.db.ExecContext(ctx,
			"DELETE FROM traces WHERE started_at < ?", cutoff)
		if err != nil {
			m.logger.Warn("failed to clean trace spans", "err", err)
			m.lastErr = fmt.Errorf("trace cleanup: %w", err)
		} else {
			n, _ := result.RowsAffected()
			traceDeleted = int(n)
		}
	}

	if m.policy.AuditTTL > 0 {
		cutoff := time.Now().Add(-m.policy.AuditTTL)
		// Verify the chain BEFORE the copy. If verification fails,
		// skip the archive this cycle; the operator should investigate.
		if _, err := m.verifyChainForRetention(ctx); err != nil {
			m.logger.Warn("retention: chain verification failed; skipping audit archive",
				"err", err)
			m.lastErr = fmt.Errorf("audit chain verify: %w", err)
		} else {
			// INSERT OR IGNORE: audit_archive.id is the PK, so a
			// re-run after a partial failure would otherwise
			// abort the entire sweep on a duplicate-id error.
			result, err := m.db.ExecContext(ctx, `
				INSERT OR IGNORE INTO audit_archive
				    (id, user_id, action, resource, details, timestamp,
				     previous_hash, entry_hash, timestamp_str,
				     resource_kind, resource_id, archived_at)
				SELECT id, user_id, action, resource, details, timestamp,
				       previous_hash, entry_hash, timestamp_str,
				       resource_kind, resource_id, CURRENT_TIMESTAMP
				FROM audit_entries
				WHERE timestamp < ?`, cutoff)
			if err != nil {
				m.logger.Warn("failed to archive audit rows", "err", err)
				m.lastErr = fmt.Errorf("audit archive: %w", err)
			} else {
				n, _ := result.RowsAffected()
				auditArchived = int(n)
			}
		}
	}

	if traceDeleted > 0 || auditArchived > 0 {
		m.logger.Info("retention cleanup completed",
			"traces_deleted", traceDeleted,
			"audit_archived", auditArchived,
			"trace_ttl", m.policy.TraceTTL,
			"audit_ttl", m.policy.AuditTTL)
	}

	if m.lastErr != nil {
		return fmt.Errorf("retention: %w", m.lastErr)
	}
	return nil
}

// verifyChainForRetention runs VerifyAuditChainOnDB and treats any
// failure as a blocker. Used by Enforce before copying audit
// rows to audit_archive — if the chain is broken we leave the
// source rows alone so the operator can investigate.
func (m *Manager) verifyChainForRetention(ctx context.Context) (string, error) {
	res, err := store.VerifyAuditChainOnDB(ctx, m.db)
	if err != nil {
		return "", err
	}
	if !res.Ok {
		return "", fmt.Errorf("chain verify failed: %s", res.Reason)
	}
	return "ok", nil
}

// GetPolicy returns the current retention policy.
func (m *Manager) GetPolicy() Policy {
	return m.policy
}
