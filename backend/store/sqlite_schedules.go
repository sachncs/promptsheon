package store
import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/backend/errs"
	"github.com/sachncs/promptsheon/backend/schedule"
)

// SQLite persistence for schedules.

func (s *SQLite) CreateSchedule(ctx context.Context, sc *schedule.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedules (id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.WorkspaceID, sc.ReleaseID, string(sc.Kind), sc.Cron, sc.WebhookPath,
		sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.CreatedAt, sc.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert schedule: %w", err)
	}
	return nil
}

// ListDueSchedules returns schedules due to fire at-or-before now.
//
// Webhook and manual schedules have no NextFireAt semantics —
// they are driven by external events, not by the cron tick.
// Returning them from the tick list would cause the scheduler to
// fire them on every tick (NextFireAt defaults to the epoch and
// never advances for non-cron kinds), creating runaway execution
// traffic. We filter them out here; webhook fires go through the
// dedicated HTTP handler and manual fires go through the CLI.

func (s *SQLite) ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]*schedule.Schedule, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by
		 FROM schedules WHERE enabled = 1 AND kind = 'cron' AND next_fire_at <= ? ORDER BY next_fire_at ASC LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list due schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*schedule.Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ClaimDueSchedule atomically transitions a schedule from
// "due" to "in-flight" by advancing next_fire_at to the next
// computed fire time and stamping last_fire_at. Returns false
// (and nil error) when the row is no longer due — the caller
// raced with another scheduler and should skip publication.
//
// The previous code path used ListDueSchedules followed by a
// separate UpdateSchedule, allowing two schedulers to both
// publish the same schedule event. ClaimDueSchedule makes the
// read+write atomic.

func (s *SQLite) ClaimDueSchedule(ctx context.Context, sc *schedule.Schedule, newNextFireAt time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = fired_count + 1, enabled = ?
		 WHERE id = ? AND next_fire_at = ?`,
		newNextFireAt, sc.LastFireAt, sc.Enabled, sc.ID, sc.NextFireAt,
	)
	if err != nil {
		return false, fmt.Errorf("claim due schedule: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim due schedule rows: %w", err)
	}
	return n == 1, nil
}


func (s *SQLite) UpdateSchedule(ctx context.Context, sc *schedule.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = ?, enabled = ?
		 WHERE id = ?`,
		sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.ID,
	)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	return nil
}

// BulkUpdateSchedules persists a batch of schedule updates in a
// single transaction. PERF-SCH-1: TickOnce uses this instead of
// looping per-row UPDATE, dropping the round-trip count from N
// to 1.

func (s *SQLite) BulkUpdateSchedules(ctx context.Context, scs []*schedule.Schedule) error {
	if len(scs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk update tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = ?, enabled = ?
		 WHERE id = ?`,
	)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare bulk update: %w", err)
	}
	defer stmt.Close()
	for _, sc := range scs {
		if _, err := stmt.ExecContext(ctx, sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.ID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("bulk update %s: %w", sc.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk update: %w", err)
	}
	return nil
}


func scanSchedule(scanner interface {
	Scan(dest ...any) error
}) (*schedule.Schedule, error) {
	var sc schedule.Schedule
	var kindStr string
	err := scanner.Scan(
		&sc.ID, &sc.WorkspaceID, &sc.ReleaseID, &kindStr, &sc.Cron, &sc.WebhookPath,
		&sc.NextFireAt, &sc.LastFireAt, &sc.FiredCount, &sc.Enabled,
		&sc.CreatedAt, &sc.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan schedule: %w", err)
	}
	sc.Kind = schedule.Kind(kindStr)
	return &sc, nil
}


