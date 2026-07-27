// Package postgres is the Postgres-backed Repository adapter
// for Promptsheon. The schema mirrors the SQLite backend; the
// row-level security policies in migrations/010_rls.up.sql
// enforce per-Workspace isolation at the database boundary.
//
// PG-1 (deferred): the production wiring uses pgx/v5 via
// database/sql. The pgx driver is intentionally NOT in
// go.mod yet; this package ships an in-memory implementation
// behind the same store.Repositories interface so the
// contract is testable end-to-end without a live Postgres
// instance. Wiring pgx is a follow-on that swaps the
// NewPostgresRepositories constructor for a real one.
//
// Until pgx lands, the package exports:
//   - NewInMemoryPostgresRepositories() *store.Repositories
//   - LoadSQL() (initSQL, rlsSQL string)
//
// so callers can inspect the SQL and the in-memory mock.
//
// PG-2 (shipped): the RLS policies in migrations/010_rls.up.sql
// are real Postgres SQL. A deployment that runs them against a
// live Postgres with `ALTER TABLE ... ENABLE ROW LEVEL
// SECURITY` enforces Workspace isolation at the database
// boundary — no application code path can read a row from
// another Workspace, even with a bug.
package postgres

import (
	"embed"
	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// LoadSQL returns the bundled SQL the Postgres backend ships
// with. The init SQL is the schema; the rls SQL is the
// row-level-security policy bundle. Both are returned as
// strings so callers can `psql -f`-style apply them, or feed
// them through a migration runner.
//
// The SQL lives in the migrations/ subdirectory as plain
// .sql files so operators can grep them and feed them to
// their migration runner of choice. The constants below are
// read by the test suite; production code reads the files
// off disk via LoadSQLFiles.
func LoadSQL() (initSQL, rlsSQL string) {
	return LoadSQLFiles()
}

// LoadSQLFiles reads the bundled SQL from disk. Production
// wiring calls this once at boot. The function is exported
// so operators can curl the bundle for offline review.
func LoadSQLFiles() (initSQL, rlsSQL string) {
	initBytes, err := migrationsFS.ReadFile("migrations/000_init.up.sql")
	if err != nil {
		panic("postgres: missing migrations/000_init.up.sql: " + err.Error())
	}
	rlsBytes, err := migrationsFS.ReadFile("migrations/010_rls.up.sql")
	if err != nil {
		panic("postgres: missing migrations/010_rls.up.sql: " + err.Error())
	}
	return string(initBytes), string(rlsBytes)
}

// InMemoryPostgres is an in-memory Repository implementation
// that satisfies the same interfaces as the SQLite backend.
// It is intended for:
//   - unit tests that exercise the Repository contract without
//     a SQLite dependency;
//   - Postgres-portability regression tests that run against
//     a stand-in before the live pgx wiring lands.
//
// The InMemory implementation does NOT enforce RLS (RLS is a
// Postgres feature, not a Go-level one); deployments that need
// multi-tenant isolation must run the real Postgres backend
// with the migrations applied.
type InMemoryPostgres struct {
	workspaces    map[string]capability.Workspace
	projects      map[string]capability.Project
	capabilities  map[string]capability.Capability
	versions      map[string]capability.Version
	releases      map[string]release.Release
	approvals     map[string]approval.Approval
	datasets      map[string]*harness.Dataset
	cases         map[string][]harness.DatasetCase
	preconditions map[string]*harness.Precondition
	evalRuns      map[string]*harness.EvalRun
	evalResults   map[string][]harness.EvalResult
	systemConfig  map[string]settings.CRDTRecord
}

// NewInMemoryPostgres constructs an empty in-memory Postgres
// backend.
func NewInMemoryPostgres() *InMemoryPostgres {
	return &InMemoryPostgres{
		workspaces:    map[string]capability.Workspace{},
		projects:      map[string]capability.Project{},
		capabilities:  map[string]capability.Capability{},
		versions:      map[string]capability.Version{},
		releases:      map[string]release.Release{},
		approvals:     map[string]approval.Approval{},
		datasets:      map[string]*harness.Dataset{},
		cases:         map[string][]harness.DatasetCase{},
		preconditions: map[string]*harness.Precondition{},
		evalRuns:      map[string]*harness.EvalRun{},
		evalResults:   map[string][]harness.EvalResult{},
		systemConfig:  map[string]settings.CRDTRecord{},
	}
}

// AsRepositories returns the in-memory backend as the
// Repositories facade the rest of the daemon consumes. The
// returned facade satisfies every interface the SQLite
// implementation satisfies (Users, APIKeys, Audit,
// ProviderKeys, Alerting, Webhooks, VaultState, WSState,
// EnforcerState, Settings, Lifecycle, CapabilityRepository,
// ReleaseRepository, ApprovalRepository, HarnessRepository).
//
// Most methods are unimplemented (returning ErrNotImplemented)
// because this is a contract test, not a feature. The few
// methods that ARE implemented (settings, lifecycle, capability
// reads) are exercised by the contract tests.
func (p *InMemoryPostgres) AsRepositories() *store.Repositories {
	return &store.Repositories{
		Users:                &usersAdapter{p: p},
		APIKeys:              &apiKeysAdapter{p: p},
		Audit:                &auditAdapter{p: p},
		ProviderKeys:         &providerKeysAdapter{p: p},
		Alerting:             &alertingAdapter{p: p},
		Webhooks:             &webhooksAdapter{p: p},
		VaultState:           &vaultStateAdapter{p: p},
		WSState:              &wsStateAdapter{p: p},
		EnforcerState:        &enforcerStateAdapter{p: p},
		Settings:             &settingsAdapter{p: p},
		Lifecycle:            &lifecycleAdapter{p: p},
		CapabilityRepository: &capabilityAdapter{p: p},
		ReleaseRepository:    &releaseAdapter{p: p},
		ApprovalRepository:   &approvalAdapter{p: p},
		HarnessRepository:    &harnessAdapter{p: p},
	}
}
