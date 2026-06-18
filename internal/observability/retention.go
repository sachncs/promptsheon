// Package observability provides log retention and cleanup policies.
package observability

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// RetentionPolicy defines TTL for different log types.
type RetentionPolicy struct {
	TraceTTL      time.Duration
	SnapshotTTL   time.Duration
	AuditTTL      time.Duration
	CheckInterval time.Duration
}

// DefaultRetentionPolicy returns sensible defaults.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		TraceTTL:      30 * 24 * time.Hour, // 30 days minimum
		SnapshotTTL:   30 * 24 * time.Hour, // 30 days
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
	if v := os.Getenv("PROMPTSHEON_SNAPSHOT_TTL_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days >= 1 {
			p.SnapshotTTL = time.Duration(days) * 24 * time.Hour
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
	db     *sql.DB
	policy RetentionPolicy
	logger *slog.Logger
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

// Enforce deletes expired data based on the retention policy.
func (m *RetentionManager) Enforce(ctx context.Context) error {
	var totalDeleted int

	// Clean trace spans
	if m.policy.TraceTTL > 0 {
		cutoff := time.Now().Add(-m.policy.TraceTTL)
		result, err := m.db.ExecContext(ctx,
			"DELETE FROM traces WHERE started_at < ?", cutoff)
		if err != nil {
			m.logger.Warn("failed to clean trace spans", "err", err)
		} else {
			n, _ := result.RowsAffected()
			totalDeleted += int(n)
		}
	}

	// Clean snapshots
	if m.policy.SnapshotTTL > 0 {
		cutoff := time.Now().Add(-m.policy.SnapshotTTL)
		result, err := m.db.ExecContext(ctx,
			"DELETE FROM output_snapshots WHERE created_at < ?", cutoff)
		if err != nil {
			m.logger.Warn("failed to clean snapshots", "err", err)
		} else {
			n, _ := result.RowsAffected()
			totalDeleted += int(n)
		}
	}

	// Clean audit entries (only old completed/failed entries, keep important ones)
	if m.policy.AuditTTL > 0 {
		cutoff := time.Now().Add(-m.policy.AuditTTL)
		result, err := m.db.ExecContext(ctx,
			`DELETE FROM audit_entries WHERE timestamp < ? 
			 AND action NOT IN ('auth_failure', 'auto_approve', 'deploy')`, cutoff)
		if err != nil {
			m.logger.Warn("failed to clean audit entries", "err", err)
		} else {
			n, _ := result.RowsAffected()
			totalDeleted += int(n)
		}
	}

	if totalDeleted > 0 {
		m.logger.Info("retention cleanup completed", "deleted", totalDeleted,
			"trace_ttl", m.policy.TraceTTL,
			"snapshot_ttl", m.policy.SnapshotTTL,
			"audit_ttl", m.policy.AuditTTL)
	}

	return nil
}

// GetPolicy returns the current retention policy.
func (m *RetentionManager) GetPolicy() RetentionPolicy {
	return m.policy
}
