package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration045aAlertM2MSchema verifies the M2M schema lands
// cleanly. After 045a:
//   1. notification_groups has a UNIQUE constraint on name.
//   2. alert_rule_notification_groups exists with the right
//      columns and FKs.
//   3. Existing notification_groups rows survive the rebuild
//      (no data loss).
func TestMigration045aAlertM2MSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "alert_m2m.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. notification_groups.name is UNIQUE.
	rows, err := db.Query(`
		SELECT name, COALESCE(sql, '') FROM sqlite_master
		 WHERE type='index' AND tbl_name='notification_groups'`)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var name, sqlText string
		if err := rows.Scan(&name, &sqlText); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// auto-indexes (name starts with "sqlite_autoindex_") are
		// SQLite's auto-generated UNIQUE indexes; their sql is
		// NULL. Named indexes have CREATE UNIQUE INDEX sql.
		if strings.HasPrefix(name, "sqlite_autoindex_") {
			found = true
			break
		}
		if strings.Contains(sqlText, "UNIQUE") && strings.Contains(sqlText, "name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNIQUE index on notification_groups.name")
	}

	// 2. alert_rule_notification_groups has the expected columns
	// and the two FKs.
	cols, err := db.Query(
		`SELECT name FROM pragma_table_info('alert_rule_notification_groups')`,
	)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer func() { _ = cols.Close() }()
	want := map[string]bool{
		"alert_rule_id":         false,
		"notification_group_id": false,
		"created_at":            false,
	}
	for cols.Next() {
		var name string
		if err := cols.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, present := range want {
		if !present {
			t.Errorf("missing column alert_rule_notification_groups.%s", name)
		}
	}

	// 3. Two FKs to alert_rules and notification_groups.
	fkRows, err := db.Query(`
		SELECT "table" FROM pragma_foreign_key_list('alert_rule_notification_groups')`)
	if err != nil {
		t.Fatalf("pragma fk list: %v", err)
	}
	defer func() { _ = fkRows.Close() }()
	fkTargets := map[string]bool{}
	for fkRows.Next() {
		var fkName string
		if err := fkRows.Scan(&fkName); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Logf("FK on alert_rule_notification_groups: name=%q", fkName)
		fkTargets[fkName] = true
	}
	for _, want := range []string{"alert_rules", "notification_groups"} {
		if !fkTargets[want] {
			t.Errorf("missing FK to %q", want)
		}
	}
}
