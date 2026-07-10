// ADR 0015: Postgres as a first-class backend, alongside SQLite
//
// - **Status:** Accepted
// - **Date:** 2026-07-10
// - **Supersedes:** —
// - **Superseded by:** —
//
// ## Context
//
// The system must serve tenants that exceed SQLite's single-writer
// limit, expect horizontal scaling, and require per-workspace row
// level security enforced at the database. SQLite continues to be the
// zero-dependency default for development and single-node
// deployments; Postgres is the production target.
//
// The consumer-defined Repository interfaces defined in
// internal/capability, internal/release, internal/approval,
// internal/recommendation, internal/lineage, and internal/policy are
// the contract between domain code and storage. A new
// internal/store/postgres package implements every interface; the
// SQLite implementation in internal/store remains unchanged.
//
// ## Decision
//
// 1. Config gains DBBackend ('sqlite' | 'postgres') and DBDSN
//    fields. The default is sqlite. Production operators set
//    PROMPTSHEON_DB_BACKEND=postgres and PROMPTSHEON_DB_DSN to the
//    cluster connection string.
// 2. The Postgres implementation uses modernc.org/pgx (pure Go, no
//    CGO), mirroring the SQLite backend's ADR-0006 choice and
//    keeping cross-compilation trivial.
// 3. Migrations live in internal/store/migrations/postgres and
//    ship in lockstep with the SQLite migrations; the
//    `internal/store/migrate` runner learns to dispatch to the
//    correct file set based on DBBackend.
// 4. Per-workspace RLS is enabled by migration 100_workspace_rls.sql,
//    which sets `ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;`
//    and creates `workspace_members` lookup policies. The test
//    `internal/store/postgres/rls_test.go` (gated on
//    PROMPTSHEON_RUN_PG_TESTS=1) connects to a live Postgres in CI
//    and proves queries filter without WHERE clauses.
// 5. RLS binds to a per-connection `SET LOCAL app.current_workspace =
//    <id>`. The Postgres implementation wraps every Repository
//    call to set this on the same transaction the call uses, so a
//    query that escapes its workspace fails closed.
//
// ## Options considered
//
// 1. **Switch the default to Postgres and require operators to opt
//    into SQLite.** Rejected: SQLite remains valuable for
//    development, single-node deployments, and CI. SQLite stays the
//    default; Postgres is opt-in via DBBackend.
// 2. **Hide RLS behind a `workspace_id` column on every table and
//    trust the application to filter.** Rejected: RLS belongs at the
//    database. The audit chain (ADR-0003), the per-tenant cost
//    model, and the SOC2 / HIPAA positioning all depend on
//    defense-in-depth at the database.
// 3. **Adopt a different Go driver (jackc/pgx/v5).** The v5 driver
//    is the canonical Postgres driver today. modernc.org/pgx is
//    the pure-Go port that pairs with the modernc.org/sqlite
//    decision already made in ADR-0006 — staying with one vendor
//    family keeps the build matrix simple. If modernc.org/pgx
//    proves unstable in CI we will revisit; the consumer-defined
//    interfaces keep that swap local to this package.
//
// ## Consequences
//
// Positive:
// - Production tenants can ship a deployment with Postgres-only
//   persistence, per-workspace RLS, and existing audit-chain
//   guarantees.
// - Domain code is unaffected: every consumer still depends only
//   on the consumer-defined Repository interfaces.
// - CI matrix gains a backend dimension and exercises both
//   backends on each PR; the SQLite tests remain the default for
//   developer loops.
//
// Negative:
// - Two storage backends, two migration sets, two test surfaces.
//   This is real maintenance cost. We offset by sharing the
//   consumer-defined interfaces and by extracting anything that can
//   share between backends (audit chain, hook storage, oid helpers)
//   into internal/store/migrate or pkg/storage later.
// - RLS requires a workspace_id column on every per-tenant table
//   with policies added by migration 100. We accept the migration
//   cost because the alternative is application-level filtering,
//   which cannot defend against bugs or accidental direct SQL access.
//
// ## References
//
// - internal/store/postgres/ (this ADR's directory target)
// - internal/store/migrations/postgres (SQL migration files)
// - internal/config (DBBackend)
// - internal/capability, internal/release, internal/approval,
//   internal/recommendation, internal/lineage, internal/policy
//   (consumer-defined Repository interfaces)
// - ADR-0006 (modernc.org/sqlite)
// - ADR-0003 (hash-chained audit log)
//