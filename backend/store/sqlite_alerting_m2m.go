package store

import (
	"context"
	"fmt"
)

// LinkRuleToGroup creates a row in the alert_rule_notification_groups
// M2M table. Idempotent: re-linking an existing pair is a no-op.
// DB-11b: split out from sqlite.go into its own file so the
// repository surface that manages alert M2M lives in one place
// and the HTTP layer can wire routes against it without grepping
// for the methods.
func (s *SQLite) LinkRuleToGroup(ctx context.Context, ruleID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO alert_rule_notification_groups (alert_rule_id, notification_group_id)
		VALUES (?, ?)`,
		ruleID, groupID,
	)
	if err != nil {
		return fmt.Errorf("link rule to group: %w", err)
	}
	return nil
}

// UnlinkRuleFromGroup removes a row from the
// alert_rule_notification_groups M2M table. Returns nil whether
// or not the row existed. DB-11b.
func (s *SQLite) UnlinkRuleFromGroup(ctx context.Context, ruleID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM alert_rule_notification_groups
		 WHERE alert_rule_id = ? AND notification_group_id = ?`,
		ruleID, groupID,
	)
	if err != nil {
		return fmt.Errorf("unlink rule from group: %w", err)
	}
	return nil
}
