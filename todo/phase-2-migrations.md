# Phase 2 — Migration Chain Repair

All schema issues. Fast forward: rewrite or drop, no compatibility shims.

## Critical

- [x] **DB-1** Rewrite migration 050 in SQLite-compatible form. (See Phase 0.)
- [x] **DB-2** Replace destructive-migration heuristic so 044 is caught. (See Phase 0.)
- [x] **DB-3** Rebuild migration 043 outside transactions. (See Phase 0.)
- [x] **DB-4** Update `recommendation.SQLiteRepository.CreateDecision` to supply a UUID `id` and remove the nullable `TEXT PRIMARY KEY` from `decisions`.
  - **Where**: `internal/recommendation/sqlite.go:131-135` and `internal/store/migrations/049_decisions_uuid_pk.up.sql`.
  - **Accept**: After `CreateDecision`, the row's `id` matches the value passed in.

## FK hygiene

- [ ] **DB-5a** Add `api_keys(user_id, created_at DESC)` index.
  - **Where**: new migration `internal/store/migrations/055_api_keys_user_index.up.sql`.

- [x] **DB-6** Seed system user `id="api"`. (See Phase 1 SEC-DB-1.)

- [ ] **DB-7** Have `VerifyAuditChain` cross-check against `audit_chain_state` and `audit_entries` rowid sequence. (See Phase 1 SEC-CHAIN-1.)

- [x] **DB-15** Drop unused tables `guardrail_rules` and `guardrail_violations` (manager is in-memory).
  - **Where**: new migration `internal/store/migrations/056_drop_guardrail_tables.up.sql`.

- [x] **DB-19** Add FK `releases.capability_id, capability_version → capability_versions(capability_id, version)`.
  - **Where**: new migration `internal/store/migrations/057_releases_version_fk.up.sql`.

- [ ] **DB-20** Add FK `eval_results.case_id → dataset_cases.id` with `ON DELETE SET NULL`.
  - **Where**: new migration `internal/store/migrations/058_eval_results_case_fk.up.sql`.

## Indexes

- [ ] **DB-8a** Add `idx_audit_resource_kind_id_time` on `(resource_kind, resource_id, timestamp DESC)`.
  - **Where**: new migration `internal/store/migrations/059_audit_resource_kind_index.up.sql`.

- [ ] **DB-8b** Update `ListAudit` filter struct to accept `resource_kind` + `resource_id`; default to legacy `resource` when both are empty.
  - **Where**: `internal/store/sqlite.go:282-288` and `internal/api/handlers_audit.go`.

- [ ] **DB-16a** Drop redundant indexes: `idx_eval_runs_release_started`, `idx_versions_capability_version_desc`, `idx_users_email`, `idx_api_keys_hash`, `idx_webhook_endpoints_active`.
  - **Where**: new migration `internal/store/migrations/060_drop_redundant_indexes.up.sql`.

## Hot-path SQL

- [x] **DB-9a** Cap `Limit` to at least 1 before building the SQL; emit `LIMIT -1 OFFSET ?` for `Offset > 0, Limit == 0` cases.
  - **Where**: `internal/store/sqlite.go:282-288` and `internal/store/sqlite_capabilities.go:403-409`.
  - **Accept**: A repository call with `Offset=10, Limit=0` runs without SQL syntax error.

- [x] **DB-9b** Add tests covering the offset-only case for `ListAudit` and `ListExecutions`.

## Alert routing

- [x] **DB-10a** Change `GetChannelsForAlertRule` to `SELECT json_each.value FROM alert_notification_groups ang JOIN json_each(ang.channels)` (flatten the JSON array into rows).
  - **Where**: `internal/store/sqlite.go:840-853`.
  - **Accept**: An alert rule with `channels: ["webhook", "log"]` returns two rows, one per channel.

- [x] **DB-10b** Deduplicate and reorder the channel union on the manager side.
  - **Where**: `internal/alerting/manager.go:398-406`.

- [x] **DB-11a** Replace `INSERT OR REPLACE` on `notification_groups` with `INSERT ... ON CONFLICT (id) DO UPDATE SET name=excluded.name, channels=excluded.channels`.
  - **Where**: `internal/store/sqlite.go:774-783`.
  - **Accept**: Updating a group's name does not cascade-delete its M2M links.

- [ ] **DB-11b** Add a repository method `LinkRuleToGroup(ctx, ruleID, groupID)` and `UnlinkRuleFromGroup` so the HTTP layer can manage M2M.
  - **Where**: new file `internal/store/sqlite_alerting_m2m.go` and `internal/api/server.go:423-432`.

## Migration numbering

- [ ] **DB-GAP-1** Re-number migrations 041-052 to fill the 028-040 gap, OR add a CHANGELOG entry explaining the gap was a numbering mistake.
  - **Where**: `internal/store/migrations/` directory.

- [ ] **DB-GAP-2** Update `052_audit_backfill_tool_marker.up.sql` comment to say "Migration 052" not "Migration 048b".
  - **Where**: `internal/store/migrations/052_audit_backfill_tool_marker.up.sql:1`.

## Down migrations

- [ ] **DB-13** Decide: ship a `down` runner or delete all `.down.sql` files.
  - **Where**: `internal/store/migrate.go:70-77` and every `*.down.sql`.

- [ ] **DB-14a** Fix `044_legacy_drop.down.sql` so its header comment matches what it actually does (currently claims to re-add columns it never adds).

- [ ] **DB-14b** Fix `046_alert_m2m_backfill.down.sql` to preserve operator-created M2M rows (currently deletes them).

## Test infrastructure

- [ ] **DB-17** Refactor migration tests so each test starts from a known version, applies only its target migration, and asserts.
  - **Where**: every `internal/store/0??_*_test.go` file.

## Schema revisions for forward-only

- [ ] **DB-REV-1** Drop the entire `prompts`, `agents`, `contexts`, `workflows`, `workflow_steps`, `prompt_versions`, `reviews`, `execution_logs` tables in one migration (no `IF EXISTS` checks — they were dropped in 044, this is for clarity).
  - **Where**: new migration `internal/store/migrations/061_drop_legacy_redundant.up.sql` (no-op on current DB; documents intent).

- [ ] **DB-REV-2** Add a `schema_version` row with explicit `feature_flags TEXT` so future feature toggles don't bloat the env.
  - **Where**: new migration `internal/store/migrations/062_feature_flags.up.sql`.

- [ ] **DB-REV-3** Replace `idx_eval_runs_status` (if any) with a covering `(release_id, status, started_at DESC)` index.
  - **Where**: `internal/store/migrations/063_eval_runs_covering.up.sql`.

## Hot-path concurrency

- [ ] **DB-CONC-1** Enable SQLite WAL mode and `busy_timeout=5000` at `NewSQLite` time.
  - **Where**: `internal/store/sqlite.go` constructor.

- [ ] **DB-CONC-2** Verify the retention loop uses a separate `*sql.DB` so cleanup never blocks request-path writes.
  - **Where**: `cmd/promptsheond/main.go:195-204, 218-220`.
