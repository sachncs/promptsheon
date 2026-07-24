# Migrations

The schema lives in 8 forward-only `.up.sql` files at versions
001..008, followed by individual `.up.sql` / `.down.sql` files at
versions 009..017. Every version prefix is a strict, unique
integer (no `014b`-style suffixes); the runner parses the leading
integer and uses it as the `schema_migrations.version` primary
key, so a suffixed name collides with the bare integer (014b ==
14 == 014). The runner skips destructive migrations unless
`PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true`.

## File layout

| File | Purpose |
|------|---------|
| `001_core_schema.up.sql` | All 24 modern tables with FKs, CHECKs, UNIQUE, basic indexes |
| `002_audit_chain.up.sql` | Audit chain hash infrastructure + append-only trigger |
| `003_indexes.up.sql` | All covering / performance indexes |
| `004_data_cleanup.up.sql` | Backfills: enum coercion, decisions NULL purge, M2M backfill, release version backfill |
| `005_seed.up.sql` | System user seed + default feature flags |
| `006_security.up.sql` | Webhook secret ciphertext (no-op on fresh installs; declarative in 001) |
| `007_views_and_triggers.up.sql` | Reserved for future views/triggers |
| `008_destructive_cleanup.up.sql` | Drop pre-v0.1.0 legacy tables; gated by env var |
| `009_vault_state.up.sql` / `.down.sql` | Singleton vault-state row (KMS rotation) |
| `010_ws_state.up.sql` / `.down.sql` | WebSocket session state |
| `011_audit_archive.up.sql` / `.down.sql` | `audit_archive` table + chain-state tail cache columns |
| `012_enforcer_state.up.sql` / `.down.sql` | Policy enforcer per-key state |
| `013_idempotency_cache.up.sql` / `.down.sql` | Idempotency-key dedupe cache |
| `014_system_config.up.sql` / `.down.sql` | Operator-tunable key/value settings table |
| `015_seed_settings.up.sql` / `.down.sql` | Seed `otl.*` / `llm.*` defaults |
| `016_bandit_arm_counters.up.sql` / `.down.sql` | Per-(arm, replica) grow-only CRDT counters |
| `017_system_config_crdt.up.sql` / `.down.sql` | LWW register metadata on `system_config` |

The destructive gate regex is `^\d+_destructive`. Only files
matching that anchored pattern are gated.

## Upgrade procedure (existing deployments)

For deployments upgrading from pre-consolidation schema
(versions 1..27, 41..64), run the one-time shim before starting
the new daemon:

```sql
INSERT OR IGNORE INTO schema_migrations (version) VALUES
    (1), (2), (3), (4), (5), (6), (7), (8);
```

This records the consolidated migration versions as applied,
so the runner skips them on next start. The legacy tables
that need to be dropped manually (per the old 044 cleanup)
should be dropped before the shim if the operator wants a
clean schema; otherwise they remain as dead tables.

## Adding a new migration

After this consolidation, every new schema change adds a file
beyond 008. Phase 2/3 changes land at `009_*.up.sql`,
`010_*.up.sql`, and so on. The destructive gate stays anchored:
a new file named `015_destructive_state_change.up.sql` is NOT
gated; a file named `015_destructive_*.up.sql` IS gated.
**Do not use suffix-style names like `014b_*.up.sql`** — the
runner only parses the leading integer and would treat `014b`
as `14`, colliding with the `014_` row.

## `migrateUpTo` targets

The `migrateUpTo(db, target)` helper accepts:
- `1` — only `001_core_schema.up.sql` (creates the modern tables)
- `2` — through `002_audit_chain.up.sql`
- `3` — through `003_indexes.up.sql`
- `4` — through `004_data_cleanup.up.sql`
- `5` — through `005_seed.up.sql`
- `6` — through `006_security.up.sql` (no-op on fresh installs)
- `7` — through `007_views_and_triggers.up.sql` (no-op)
- `8` — through `008_destructive_cleanup.up.sql` (gated)
- `14` — through `014_system_config.up.sql`
- `15` — through `015_seed_settings.up.sql`
- `16` — through `016_bandit_arm_counters.up.sql`
- `17` — through `017_system_config_crdt.up.sql`
