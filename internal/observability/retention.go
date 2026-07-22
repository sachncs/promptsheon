// Package observability provides log retention and cleanup policies.
package observability

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/sachncs/promptsheon/internal/store"
)

// RetentionPolicy defines TTL for different log types.
type RetentionPolicy struct {
	TraceTTL      time.Duration
	AuditTTL      time.Duration
	CheckInterval time.Duration
}

// DefaultRetentionPolicy returns sensible defaults.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		TraceTTL:      30 * 24 * time.Hour, // 30 days minimum
		AuditTTL:      90 * 24 * time.Hour, // 90 days
		CheckInterval: 1 * time.Hour,       // check every hour
	}
}

// LoadRetentionPolicyFromEnv loads retention policy from environment variables.
// Supported env vars: PROMPTSHEON_TRACE_TTL_DAYS, PROMPTSHEON_SNAPSHOT_TTL_DAYS,
// PROMPTSHEON_AUDIT_TTL_DAYS, PROMPTSHEON_RETENTION_CHECK_MINUTES.
func LoadRetentionPolicyFromEnv() RetentionPolicy {
	p := DefaultRetentionPolicy()

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

// RetentionManager handles periodic cleanup of expired data.
type RetentionManager struct {
	db      *sql.DB
	policy  RetentionPolicy
	logger  *slog.Logger
	lastErr error
}

// NewRetentionManager creates a retention manager.
func NewRetentionManager(db *sql.DB, policy RetentionPolicy, logger *slog.Logger) *RetentionManager {
	return &RetentionManager{
		db:     db,
		policy: policy,
		logger: logger,
	}
}

// Start begins the periodic cleanup goroutine.
func (m *RetentionManager) Start(ctx context.Context) {
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

// protectedAuditActions is retained as a documented invariant even
// though audit retention is now a no-op: the set names every action
// that the security review classified as "must never be lost". The
// table is exported through this variable so future audit-archive
// tooling can read it.
//
// The previous implementation tried to honour this list by deleting
// only non-protected actions, but deletion still broke the audit
// chain (VerifyAuditChain walks the table from rowid 1 forward,
// chaining by previous_hash). Removing rows from the middle of the
// chain is unrecoverable. The current policy is therefore: never
// delete audit rows. Operators archive externally.
var protectedAuditActions = map[string]bool{
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
func (m *RetentionManager) Enforce(ctx context.Context) error {
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
			result, err := m.db.ExecContext(ctx, `
				INSERT INTO audit_archive
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
func (m *RetentionManager) verifyChainForRetention(ctx context.Context) (string, error) {
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
func (m *RetentionManager) GetPolicy() RetentionPolicy {
	return m.policy
}
