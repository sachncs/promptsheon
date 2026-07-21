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
// SECURITY: audit rows are NOT deleted, regardless of age or action.
// The audit chain walks from rowid 1 forward, chaining entries by
// previous_hash. Deleting entries in the middle of the chain breaks
// verification even though the surviving rows are intact. The chain
// is the security boundary; we keep it whole and let operators
// archive it externally (e.g. snapshotting the database).
func (m *RetentionManager) Enforce(ctx context.Context) error {
	var totalDeleted int
	m.lastErr = nil

	if m.policy.TraceTTL > 0 {
		cutoff := time.Now().Add(-m.policy.TraceTTL)
		result, err := m.db.ExecContext(ctx,
			"DELETE FROM traces WHERE started_at < ?", cutoff)
		if err != nil {
			m.logger.Warn("failed to clean trace spans", "err", err)
			m.lastErr = err
		} else {
			n, _ := result.RowsAffected()
			totalDeleted += int(n)
		}
	}

	if totalDeleted > 0 {
		m.logger.Info("retention cleanup completed", "deleted", totalDeleted,
			"trace_ttl", m.policy.TraceTTL)
	}

	// OPS-3: surface DELETE failures so the metrics collector can
	// alert on retention drift. The previous version logged and
	// returned nil, masking persistent SQLite errors as success.
	if m.lastErr != nil {
		return fmt.Errorf("retention: %w", m.lastErr)
	}
	return nil
}

// GetPolicy returns the current retention policy.
func (m *RetentionManager) GetPolicy() RetentionPolicy {
	return m.policy
}
