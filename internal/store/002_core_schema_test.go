package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/release"
)

// migrateOnce opens a fresh DB and runs the full migration set.
// Returns the *SQLite for further use.
func migrateOnce(t *testing.T) *SQLite {
	t.Helper()
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "consolidated.db")
	repo, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	if err := migrate(repo.DB(), migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo
}

// TestCoreSchemaTablesExist confirms every expected table is
// present after the consolidated 002 runs.
func TestCoreSchemaTablesExist(t *testing.T) {
	s := migrateOnce(t)
	want := []string{
		"users", "api_keys", "provider_keys",
		"webhook_endpoints", "audit_entries", "audit_chain_state",
		"workspaces", "projects", "capabilities",
		"capability_versions", "executions", "releases",
		"approvals", "schedules", "datasets", "dataset_cases",
		"preconditions", "eval_runs", "eval_results",
		"alert_rules", "alerts", "notification_groups",
		"alert_rule_notification_groups", "recommendations",
		"decisions", "lineage_edges", "feature_flags",
	}
	for _, tn := range want {
		var n int
		if err := s.DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tn,
		).Scan(&n); err != nil {
			t.Errorf("query %s: %v", tn, err)
		}
		if n == 0 {
			t.Errorf("expected table %s to exist after consolidated 002", tn)
		}
	}
}

// TestWebhookEndpointURLUnique locks in the Phase 1.3 fold.
func TestWebhookEndpointURLUnique(t *testing.T) {
	s := migrateOnce(t)
	ctx := context.Background()
	if err := s.SaveWebhookEndpoint(ctx, &models.WebhookEndpointRecord{
		ID: "w1", URL: "https://example.com/hook",
		Events: []string{"capability.created"}, Active: true,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	err := s.SaveWebhookEndpoint(ctx, &models.WebhookEndpointRecord{
		ID: "w2", URL: "https://example.com/hook",
		Events: []string{"capability.created"}, Active: true,
		CreatedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected UNIQUE(url) to reject duplicate URL")
	}
	if !strings.Contains(err.Error(), "UNIQUE") &&
		!strings.Contains(err.Error(), "constraint") {
		t.Errorf("expected UNIQUE constraint error, got %v", err)
	}
}

// TestWebhookEndpointNoSecretColumn locks in the Phase 1.4 fold.
// The plaintext `secret` column must not exist in the schema.
func TestWebhookEndpointNoSecretColumn(t *testing.T) {
	s := migrateOnce(t)
	rows, err := s.DB().Query(
		`SELECT name FROM pragma_table_info('webhook_endpoints')`,
	)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "secret" {
			t.Errorf("plaintext secret column still exists in webhook_endpoints")
		}
	}
}

// TestFeatureFlagsEnabledCheck locks in the Phase 1.6 fold.
// The CHECK constraint rejects values other than 0 or 1.
func TestFeatureFlagsEnabledCheck(t *testing.T) {
	s := migrateOnce(t)
	_, err := s.DB().Exec(
		`INSERT INTO feature_flags (name, enabled, description, updated_at)
		 VALUES ('bad', 2, 'test', CURRENT_TIMESTAMP)`,
	)
	if err == nil {
		t.Fatal("expected CHECK(enabled IN (0,1)) to reject 2")
	}
}

// TestAlertRulesEnabledCheck — same constraint on alert_rules.
func TestAlertRulesEnabledCheck(t *testing.T) {
	s := migrateOnce(t)
	_, err := s.DB().Exec(
		`INSERT INTO alert_rules (name, type, severity, enabled, threshold, duration, window, created_at, updated_at)
		 VALUES ('r', 'eval_failed', 'low', 7, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	)
	if err == nil {
		t.Fatal("expected CHECK(enabled IN (0,1)) to reject 7")
	}
}

// TestSchedulesEnabledCheck — same on schedules.
func TestSchedulesEnabledCheck(t *testing.T) {
	s := migrateOnce(t)
	_, err := s.DB().Exec(
		`INSERT INTO schedules
		   (id, workspace_id, release_id, kind, cron, next_fire_at, enabled, created_at, created_by)
		 VALUES ('s1', 'w1', 'r1', 'cron', '*/5 *', CURRENT_TIMESTAMP, 9, CURRENT_TIMESTAMP, 'api')`,
	)
	if err == nil {
		t.Fatal("expected CHECK(enabled IN (0,1)) to reject 9")
	}
}

// TestExecutionsOnDeleteSetNull locks in the Phase 1.10 fold.
func TestExecutionsOnDeleteSetNull(t *testing.T) {
	s := migrateOnce(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.CreateWorkspace(ctx, &capability.Workspace{ID: "w1", Name: "w", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "p", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := s.CreateCapability(ctx, &capability.Capability{ID: "c1", ProjectID: "p1", Name: "c", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("capability: %v", err)
	}
	if err := s.CreateVersion(ctx, &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		ManifestHash: "h1", CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("version: %v", err)
	}
	if err := s.CreateExecution(ctx, &capability.Execution{
		ID: "e1", CapabilityVersionID: "v1", Error: "",
	}); err != nil {
		t.Fatalf("execution: %v", err)
	}

	// Delete the version; the execution should remain with FK NULL.
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM capability_versions WHERE id='v1'`); err != nil {
		t.Fatalf("delete version: %v", err)
	}
	var cvid sql.NullString
	if err := s.DB().QueryRow(
		`SELECT capability_version_id FROM executions WHERE id='e1'`,
	).Scan(&cvid); err != nil {
		t.Fatalf("query: %v", err)
	}
	if cvid.Valid || cvid.String != "" {
		t.Errorf("expected capability_version_id=NULL after ON DELETE SET NULL, got %q (valid=%v)", cvid.String, cvid.Valid)
	}
}

// TestAlertsOnDeleteSetNull locks in the Phase 1.10 fold.
func TestAlertsOnDeleteSetNull(t *testing.T) {
	s := migrateOnce(t)
	ctx := context.Background()
	if _, err := s.DB().ExecContext(ctx,
		`INSERT INTO alert_rules
		   (id, name, type, severity, enabled, threshold, duration, window, created_at, updated_at)
		 VALUES ('r1', 'r', 'eval_failed', 'low', 1, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	); err != nil {
		t.Fatalf("rule: %v", err)
	}
	if _, err := s.DB().ExecContext(ctx,
		`INSERT INTO alerts
		   (rule_id, rule_name, severity, status, message)
		 VALUES ('r1', 'r', 'low', 'active', 'test')`,
	); err != nil {
		t.Fatalf("alert: %v", err)
	}

	if _, err := s.DB().ExecContext(ctx, `DELETE FROM alert_rules WHERE id='r1'`); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	var rid sql.NullString
	if err := s.DB().QueryRow(
		`SELECT rule_id FROM alerts LIMIT 1`,
	).Scan(&rid); err != nil {
		t.Fatalf("query: %v", err)
	}
	if rid.Valid || rid.String != "" {
		t.Errorf("expected rule_id=NULL after ON DELETE SET NULL, got %q (valid=%v)", rid.String, rid.Valid)
	}
}

// TestAlertsAcknowledgementColumns locks in the Phase 1.9 fold.
func TestAlertsAcknowledgementColumns(t *testing.T) {
	s := migrateOnce(t)
	cols := map[string]bool{}
	rows, err := s.DB().Query(
		`SELECT name FROM pragma_table_info('alerts')`,
	)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = true
	}
	if !cols["acknowledged_at"] {
		t.Error("alerts.acknowledged_at missing (Phase 1.9 fold)")
	}
	if !cols["acknowledged_by"] {
		t.Error("alerts.acknowledged_by missing (Phase 1.9 fold)")
	}
}

// TestLineageEdgesTypedColumns locks in the Phase 1.7 fold.
// parent/child must be typed columns, not TEXT.
func TestLineageEdgesTypedColumns(t *testing.T) {
	s := migrateOnce(t)
	cols := map[string]string{}
	rows, err := s.DB().Query(
		`SELECT name, type FROM pragma_table_info('lineage_edges')`,
	)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = typ
	}
	for _, c := range []string{
		"parent_capability_id", "parent_version",
		"child_capability_id", "child_version",
	} {
		if _, ok := cols[c]; !ok {
			t.Errorf("lineage_edges.%s missing (Phase 1.7 fold)", c)
		}
	}
	if _, ok := cols["parent"]; ok {
		t.Errorf("lineage_edges.parent (TEXT) should be removed (Phase 1.7 fold)")
	}
	if _, ok := cols["child"]; ok {
		t.Errorf("lineage_edges.child (TEXT) should be removed (Phase 1.7 fold)")
	}
}

// TestPartialUniqueActiveRelease locks in the 041 partial unique
// index lives in 002.
func TestPartialUniqueActiveRelease(t *testing.T) {
	s := migrateOnce(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.CreateWorkspace(ctx, &capability.Workspace{ID: "w1", Name: "w", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "p", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := s.CreateCapability(ctx, &capability.Capability{ID: "c1", ProjectID: "p1", Name: "c", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("capability: %v", err)
	}
	rel1 := &release.Release{
		ID:                "r1",
		CapabilityID:      "c1",
		CapabilityVersion: 1,
		Environment:       release.EnvProd,
		Status:            release.StatusActive,
		CreatedBy:         "u1",
		CreatedAt:         now,
	}
	if err := s.CreateRelease(ctx, rel1); err != nil {
		t.Fatalf("first release: %v", err)
	}
	rel2 := &release.Release{
		ID:                "r2",
		CapabilityID:      "c1",
		CapabilityVersion: 1,
		Environment:       release.EnvProd,
		Status:            release.StatusActive,
		CreatedBy:         "u1",
		CreatedAt:         now,
	}
	err := s.CreateRelease(ctx, rel2)
	if err == nil {
		t.Fatal("expected partial UNIQUE to reject second active release in same env")
	}
	if !strings.Contains(err.Error(), "UNIQUE") {
		t.Errorf("expected UNIQUE error, got %v", err)
	}
}

// TestLineageEdgesForeignKeyConstraint locks in the Phase 1.7
// typed FKs: an insert with non-existent parent should fail.
func TestLineageEdgesForeignKeyConstraint(t *testing.T) {
	s := migrateOnce(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.CreateWorkspace(ctx, &capability.Workspace{ID: "w1", Name: "w", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "p", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := s.CreateCapability(ctx, &capability.Capability{ID: "c1", ProjectID: "p1", Name: "c", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("capability: %v", err)
	}
	if err := s.CreateVersion(ctx, &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		ManifestHash: "h1", CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("version: %v", err)
	}
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO lineage_edges
		   (capability_id, parent_capability_id, parent_version, child_capability_id, child_version, source, created_by, notes)
		 VALUES ('c1', 'bogus', 1, 'c1', 1, 'manual', 'api', '{}')`,
	)
	if err == nil {
		t.Fatal("expected FK violation on bogus parent_capability_id")
	}
}

// TestErrNotFoundTranslation pins the typed error path for the
// consolidated FK rebuilds. GetEvalRun on a missing row should
// return store.ErrNotFound, not sql.ErrNoRows.
func TestErrNotFoundTranslation(t *testing.T) {
	s := migrateOnce(t)
	_, err := s.GetEvalRun(context.Background(), "no-such")
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
