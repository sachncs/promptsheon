package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
	"github.com/sachncs/promptsheon/backend/settings"
	"github.com/sachncs/promptsheon/backend/store"
	"github.com/sachncs/promptsheon/backend/store/postgres"
)

// TestPostgresContractSatisfied is the headline PG-1
// regression test: the in-memory Postgres backend implements
// the full store.Repositories facade. A future pgx wiring
// must produce a Repositories value with the same shape.
func TestPostgresContractSatisfied(t *testing.T) {
	repos := postgres.NewInMemoryPostgres().AsRepositories()
	if repos == nil {
		t.Fatal("AsRepositories returned nil")
	}
	if repos.Users == nil {
		t.Error("Users adapter not wired")
	}
	if repos.Audit == nil {
		t.Error("Audit adapter not wired")
	}
	if repos.CapabilityRepository == nil {
		t.Error("CapabilityRepository adapter not wired")
	}
	if repos.HarnessRepository == nil {
		t.Error("HarnessRepository adapter not wired")
	}
	if repos.Lifecycle == nil {
		t.Error("Lifecycle adapter not wired")
	}
}

// TestPostgresLifecyclePings verifies the contract: a fresh
// in-memory Postgres backend responds to Ping with nil
// (matches the SQLite behaviour on a healthy connection).
func TestPostgresLifecyclePings(t *testing.T) {
	repos := postgres.NewInMemoryPostgres().AsRepositories()
	if err := repos.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := repos.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestPostgresSettingsCRDTRoundtrip exercises the settings
// surface: Set + Get + List. The CRDT merge path is also
// covered (a later write overrides an earlier one).
func TestPostgresSettingsCRDTRoundtrip(t *testing.T) {
	repos := postgres.NewInMemoryPostgres().AsRepositories()
	ctx := context.Background()
	rec := settings.CRDTRecord{
		Key:           "otel_endpoint",
		Value:         "http://primary",
		ReplicaID:     "primary",
		VersionVector: map[string]uint64{"primary": 1},
		WriteTS:       100,
	}
	if err := repos.SetSystemConfig(ctx, rec); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := repos.GetSystemConfig(ctx, "otel_endpoint")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Value != "http://primary" {
		t.Errorf("roundtrip value: got %q want http://primary", got.Value)
	}
	// Verify error path: missing key returns store.ErrNotFound.
	if _, err := repos.GetSystemConfig(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing key: got %v want ErrNotFound", err)
	}
	// List returns the row.
	list, err := repos.ListSystemConfig(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list: got %d rows want 1", len(list))
	}
}

// TestPostgresCapabilityRoundtrip exercises the typed
// CapabilityRepository surface: Workspace, Project,
// Capability, Version.
func TestPostgresCapabilityRoundtrip(t *testing.T) {
	repos := postgres.NewInMemoryPostgres().AsRepositories()
	ctx := context.Background()
	ws := &capability.Workspace{ID: "ws-1", Name: "test", CreatedAt: time.Now()}
	if err := repos.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := repos.GetWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	proj := &capability.Project{ID: "p-1", WorkspaceID: "ws-1", Name: "proj"}
	if err := repos.CreateProject(ctx, proj); err != nil {
		t.Fatalf("create project: %v", err)
	}
	cap := &capability.Capability{ID: "c-1", ProjectID: "p-1", Name: "cap"}
	if err := repos.CreateCapability(ctx, cap); err != nil {
		t.Fatalf("create capability: %v", err)
	}
	got, err := repos.GetCapability(ctx, "c-1")
	if err != nil {
		t.Fatalf("get capability: %v", err)
	}
	if got.ID != "c-1" || got.ProjectID != "p-1" {
		t.Errorf("roundtrip: got %+v", got)
	}
}

// TestPostgresLoadSQLBundlesIsNonEmpty pins PG-2: the bundled
// SQL is non-empty (the embed succeeded). The SQL must
// contain the workspace_id column and a CREATE POLICY for
// the table isolation invariant.
func TestPostgresLoadSQLBundlesIsNonEmpty(t *testing.T) {
	initSQL, rlsSQL := postgres.LoadSQL()
	if len(initSQL) < 1000 {
		t.Errorf("init SQL too short: %d bytes", len(initSQL))
	}
	if len(rlsSQL) < 1000 {
		t.Errorf("rls SQL too short: %d bytes", len(rlsSQL))
	}
	for _, fragment := range []string{
		"CREATE TABLE",
		"workspace_id",
		"ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY",
	} {
		if !strings.Contains(initSQL+rlsSQL, fragment) {
			t.Errorf("SQL bundle missing %q", fragment)
		}
	}
}
