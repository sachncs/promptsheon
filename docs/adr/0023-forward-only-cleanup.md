# ADR 0023: Forward-only cleanup of the legacy bundle model

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

Per the architecture review board (v0.0.7 release), the legacy
bundle model carried Prompt / ModelPolicy / ContextContract /
Memory / Guardrails / Tools / MCPServers / RuntimePolicy /
EvaluationSuite as fields on `capability.Version`, plus a
matching column family in `capability_versions` and
`capabilities.state` / `capabilities.current_version_id` as
vestigial state. The M0.5 Manifest unification moved the
authoritative payload into a content-addressed Manifest, leaving
the legacy fields as a transition shim.

The "forward only" principle in the engineering charter rejects
deprecation paths. This ADR records the v0.1.0 commit series
that deletes every legacy shim:

- internal/promptsheon/alias.go (the M0.7 CAS re-export shim)
- internal/capability/{prompt, modelpolicy, contextcontract,
  memory, guardrail, tool, mcpserver, knowledgesource,
  runtimepolicy, evaluationsuite}.go (10 legacy bundle types)
- internal/{context, guardrail, optimizer, workflow}/
  *version*.go (the legacy analyzers that consumed the bundle
  types)
- internal/eval/runner.go's RunVersion / buildVersionPrompt
- internal/api/handlers_capabilities.go's manifestFromLegacy
- internal/{playground, collab, abtesting}/ (the dead
  playground, the unused collab package, the abtesting rename)
- internal/store/migrations/024_drop_legacy.sql +
  026_vestigial_columns.sql (the defensive migrations)
- internal/store/migrations/025_destructive.sql +
  postgres/025_destructive.sql (the destructive migration that
  drops the legacy tables and vestigial columns)
- the defensive writes of 'draft' / '' for the vestigial columns
  in sqlite_capabilities.go + postgres.go

## Decision

The v0.1.0 commit series is the single forward-only breaking
release. It is not preceded by a deprecation period; clients
that rely on the legacy fields must roll back to v0.0.7.

The Version struct is now `{ID, CapabilityID, Manifest,
ManifestHash, Version (int), CreatedAt, CreatedBy}`. The
Manifest is the only mutable payload beyond metadata. Artifacts
that previously lived as inline struct fields (Prompt text,
ModelPolicy parameters, etc.) are content-addressed and fetched
via the content-addressable store; M3 follow-on per ADR-0019
wires the production CAS path.

The API route POST `/v1/versions` accepts only `{version,
manifest}`. Missing Manifest returns 400. There is no synthesis
from a legacy bundle shape.

The capability_versions table no longer carries the legacy
columns. Migration 025 (Postgres) drops them with ALTER TABLE;
SQLite drops them via the table-rewrite-via-DROP pattern. The
`capabilities.state` and `capabilities.current_version_id`
columns are gone in the same migration.

The internal/abtesting package becomes internal/experiment per
the architecture review board §22 (\"Migrate to Manifest
references only\"). The Engine, Test, Variant, Metric types are
unchanged; only the import path moves.

## Options considered

1. **Deprecation period with both old and new paths.** Rejected.
   The charter principle is 'forward only'; deprecation is a
   backwards-compat path. Today's v0.0.7 code is still v0.0.7; it
   is a separate release line, not a path v0.1.0 supports.

2. **Two-step release (v0.0.8 keeps legacy, v0.0.9 drops it).**
   Rejected. The board calls for 'forward only' and the legacy
   fields are documented unused since M0.5/M0.8; no production
   caller uses them. Two-step rolls back the consolidation
   v0.1.0 was meant to ship.

3. **Forward-only v0.1.0 (chosen).** Single breaking release.
   The architecture review board's v0.0.7 release did not include
   the legacy shims; v0.1.0 is the first release that exercises
   the principle. Production tenants upgrade by running migration
   025 + restarting the daemon; no API change beyond the route
   parameter validation.

## Consequences

Positive:

- The Version struct's data model matches the schema, the
  storage layer, the API, the executor, the supervisor, the
  plugin SDK, the SLO library, the bandit recommender, the
  Recommendation engine, and the Manifest. There is no longer a
  secondary model. The v0.1.0 codebase is the architecture
  review board's 'Forward only' baseline.
- Storage writes are minimal: the capability_versions table
  contains (id, capability_id, version, manifest, manifest_hash,
  created_at, created_by). The defensive writes that padded
  state='draft' / current_version_id='' are gone.
- The internal/abtesting -> internal/experiment rename is
  mechanical; no API changes.

Negative:

- Production tenants running v0.0.7 must run migration 025
  before they can start the v0.1.0 daemon. There is no automatic
  backwards-compat codepath; the migration is destructive.
- v0.1.0 is a breaking semver bump. The git history is the
  source of truth; v0.0.7 is the last release that supports
  the legacy model.

## References

- Commit series: f0dd297 (F-01 shim), 68ab9ba (F-03), 9f8de04
  (F-04 + F-05), 897baab (F-06), c1936f4 (F-08), dd7d58c
  (F-09 + F-10), 33b0925 (F-07), 14288eb (F-13), eca61cd (F-14).
- ADR-0015 (Postgres+RLS), 0010 (Manifest), 0017 (Approval),
  0018 (Recommendation loop), 0019 (Deferred items), 0022
  (Plugin manifest) — the architectural decisions that
  prepared the codebase for this forward-only cleanup.
- Architecture Review §21 Tier 1.26, Tier 1.40, §22
  (Migrate / Delete lists).
