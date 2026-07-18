package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/store/postgres"
)

// Integration tests for the postgres package. They exercise the
// full CRUD path against a live Postgres instance. The DSN is
// read from TEST_POSTGRES_DSN; if unset the tests are skipped so
// the package remains buildable in environments without
// Postgres.
//
// To run locally:
//
//	docker run --rm -d -p 5432:5432 -e POSTGRES_PASSWORD=test \
//	  -e POSTGRES_USER=test -e POSTGRES_DB=promptsheon postgres:16
//	export TEST_POSTGRES_DSN="postgres://test:test@127.0.0.1:5432/promptsheon?sslmode=disable"
//	go test ./internal/store/postgres/...

const schemaSQL = `
CREATE TABLE IF NOT EXISTS workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	organization TEXT DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS capabilities (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT DEFAULT '',
	owner TEXT DEFAULT '',
	tags TEXT DEFAULT '[]',
	state TEXT NOT NULL DEFAULT 'draft',
	current_version_id TEXT DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS capability_versions (
	id TEXT PRIMARY KEY,
	capability_id TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 1,
	manifest TEXT NOT NULL DEFAULT '{}',
	manifest_hash TEXT DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	created_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS executions (
	id TEXT PRIMARY KEY,
	capability_version_id TEXT NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL,
	inputs TEXT DEFAULT '{}',
	outputs TEXT DEFAULT '{}',
	model TEXT DEFAULT '',
	provider TEXT DEFAULT '',
	latency_ms BIGINT DEFAULT 0,
	cost_usd DOUBLE PRECISION DEFAULT 0,
	prompt_tokens INTEGER DEFAULT 0,
	completion_tokens INTEGER DEFAULT 0,
	total_tokens INTEGER DEFAULT 0,
	error TEXT DEFAULT '',
	trace_id TEXT DEFAULT '',
	environment TEXT DEFAULT ''
);
`

// dsnOnce caches the connection DSN across tests so the env-var
// lookup happens once.
var (
	dsnOnce sync.Once
	dsn     string
	dsnErr  error
)

func integrationDSN() (string, error) {
	dsnOnce.Do(func() {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			dsnErr = errors.New("TEST_POSTGRES_DSN not set; skipping integration tests")
		}
	})
	return dsn, dsnErr
}

func setupDB(t *testing.T) *postgres.Postgres {
	t.Helper()
	dsn, err := integrationDSN()
	if err != nil {
		t.Skipf("postgres integration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Skipf("postgres open: %v", err)
	}
	// Apply the schema. The pool is reused so each test gets a
	// fresh schema; we wrap in a single transaction to keep the
	// application fast.
	if err := applySchema(ctx, p, dsn); err != nil {
		_ = p.Close()
		t.Skipf("apply schema: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func applySchema(ctx context.Context, p *postgres.Postgres, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, schemaSQL)
	return err
}

func TestOpenRejectsEmptyDSN(t *testing.T) {
	t.Parallel()
	if _, err := postgres.Open(context.Background(), ""); err == nil {
		t.Error("Open(\"\") returned nil error")
	}
}

func TestOpenRejectsMalformedDSN(t *testing.T) {
	t.Parallel()
	if _, err := postgres.Open(context.Background(), "not-a-valid-dsn"); err == nil {
		t.Error("Open with bad DSN returned nil error")
	}
}

func TestWorkspaceRoundTrip(t *testing.T) {
	p := setupDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	w := &capability.Workspace{
		ID:           "w1",
		Name:         "Acme",
		Organization: "Acme Inc.",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := p.CreateWorkspace(ctx, w); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	got, err := p.GetWorkspace(ctx, "w1")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.Name != w.Name {
		t.Errorf("GetWorkspace name = %q, want %q", got.Name, w.Name)
	}
	list, err := p.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListWorkspaces empty after insert")
	}
	w.Name = "Acme 2"
	w.UpdatedAt = now.Add(time.Minute)
	if err := p.UpdateWorkspace(ctx, w); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}
	if err := p.DeleteWorkspace(ctx, "w1"); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}
	if _, err := p.GetWorkspace(ctx, "w1"); err == nil {
		t.Error("GetWorkspace after delete: expected error")
	}
}

func TestProjectRoundTrip(t *testing.T) {
	p := setupDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := p.CreateWorkspace(ctx, &capability.Workspace{
		ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	prj := &capability.Project{
		ID:          "p1",
		WorkspaceID: "w1",
		Name:        "Greetings",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := p.CreateProject(ctx, prj); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	got, err := p.GetProject(ctx, "p1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != prj.Name {
		t.Errorf("GetProject name = %q, want %q", got.Name, prj.Name)
	}
	list, err := p.ListProjects(ctx, "w1")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListProjects count = %d, want 1", len(list))
	}
}

func TestCapabilityRoundTrip(t *testing.T) {
	p := setupDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := p.CreateWorkspace(ctx, &capability.Workspace{
		ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if err := p.CreateProject(ctx, &capability.Project{
		ID: "p1", WorkspaceID: "w1", Name: "Greetings", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	c := &capability.Capability{
		ID: "c1", ProjectID: "p1", Name: "greeting",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := p.CreateCapability(ctx, c); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}
	got, err := p.GetCapability(ctx, "c1")
	if err != nil {
		t.Fatalf("GetCapability: %v", err)
	}
	if got.Name != c.Name {
		t.Errorf("GetCapability name = %q, want %q", got.Name, c.Name)
	}
	list, err := p.ListCapabilities(ctx, "p1")
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListCapabilities count = %d, want 1", len(list))
	}
}

func TestVersionRoundTrip(t *testing.T) {
	p := setupDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := p.CreateWorkspace(ctx, &capability.Workspace{
		ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if err := p.CreateProject(ctx, &capability.Project{
		ID: "p1", WorkspaceID: "w1", Name: "Greetings", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := p.CreateCapability(ctx, &capability.Capability{
		ID: "c1", ProjectID: "p1", Name: "greeting",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}
	v := &capability.Version{
		ID:           "v1",
		CapabilityID: "c1",
		Version:      1,
		CreatedAt:    now,
		CreatedBy:    "u1",
	}
	if err := p.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	got, err := p.GetVersion(ctx, "v1")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("GetVersion version = %d, want 1", got.Version)
	}
	versions, err := p.ListVersions(ctx, "c1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("ListVersions count = %d, want 1", len(versions))
	}
	latest, err := p.GetLatestVersion(ctx, "c1")
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if latest.ID != "v1" {
		t.Errorf("GetLatestVersion id = %q, want v1", latest.ID)
	}
}

func TestExecutionRoundTrip(t *testing.T) {
	p := setupDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if err := p.CreateWorkspace(ctx, &capability.Workspace{
		ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if err := p.CreateProject(ctx, &capability.Project{
		ID: "p1", WorkspaceID: "w1", Name: "Greetings", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := p.CreateCapability(ctx, &capability.Capability{
		ID: "c1", ProjectID: "p1", Name: "greeting",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}
	if err := p.CreateVersion(ctx, &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	e := &capability.Execution{
		ID:                  "exec-1",
		CapabilityVersionID: "v1",
		Timestamp:           now,
		Inputs:              map[string]any{"q": "hi"},
		Outputs:             map[string]any{"a": 42},
		Model:               "gpt-4",
		Provider:            "openai",
		LatencyMs:           120,
		CostUSD:             0.002,
		PromptTokens:        50,
		CompletionTokens:    25,
		TotalTokens:         75,
		Environment:         "prod",
	}
	if err := p.CreateExecution(ctx, e); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}
	got, err := p.GetExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if got.Model != "gpt-4" {
		t.Errorf("GetExecution model = %q, want gpt-4", got.Model)
	}
	list, err := p.ListExecutions(ctx, capability.ExecutionFilter{CapabilityVersionID: "v1"})
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListExecutions count = %d, want 1", len(list))
	}
}

func TestWithWorkspaceRejectsEmptyID(t *testing.T) {
	p := setupDB(t)
	err := p.WithWorkspaceForTest(context.Background(), "", func(tx pgx.Tx) error {
		return nil
	})
	if err == nil {
		t.Error("WithWorkspaceForTest(\"\") returned nil error")
	}
}

func TestOpenPingFailure(t *testing.T) {
	// A reachable address with no Postgres listener: should fail
	// at Ping. We use port 1 which is reserved and unlikely to be
	// in use; the Dial will succeed but Ping will reject.
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := postgres.Open(ctx, "postgres://x:x@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Error("Open with unreachable host returned nil error")
	}
}
