package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/errs"
	"github.com/sachncs/promptsheon/promptsheon/models"
)

// SQLite persistence for alerting.

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

func scanAlertRule(row scannable) (*models.AlertRuleRecord, error) {
	var r models.AlertRuleRecord
	var configJSON string
	err := row.Scan(
		&r.ID, &r.Name, &r.Type, &r.Severity, &r.Enabled, &r.Threshold, &r.Duration, &r.Window,
		&configJSON, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrStoreNotFound
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
			return nil, errs.ErrStoreNotFound
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

func scanNotificationGroup(row scannable) (*models.NotificationGroupRecord, error) {
	var g models.NotificationGroupRecord
	var channelsJSON string
	err := row.Scan(&g.ID, &g.Name, &channelsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrStoreNotFound
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
