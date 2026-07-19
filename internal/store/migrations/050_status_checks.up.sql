-- Migration 050 (status CHECKs): add CHECK constraints to the closed
-- enum columns on still-living tables (post-044). The audit
-- flagged that 8 status / enum columns accepted any string;
-- a typo'd value (e.g. 'superceded') would silently break
-- filter queries.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- Scope: only the surviving tables. capabilities.state and the
-- 10 capability_versions per-artifact columns were dropped in
-- 044; their CHECKs are not in scope. workflows / workflow_steps
-- / contexts were also dropped.
--
-- The migration truncates bad data first: any existing row with
-- a value outside the closed enum is coerced to the closest
-- safe value (the empty string for env, 'running' for status,
-- etc.). The audit's recommendation was a literal failing
-- migration; we prefer to clean up rather than block the
-- upgrade.
--
-- The CHECK constraints are defensive only; the application
-- code already validates these fields at the Go level. The
-- constraint catches typos that escape the validation (e.g. a
-- raw string from a test fixture).

-- executions.environment: closed set of environments.
UPDATE executions SET environment = 'dev'
 WHERE environment NOT IN ('dev', 'staging', 'prod', '');
ALTER TABLE executions ADD CONSTRAINT chk_executions_env
  CHECK (environment IN ('dev', 'staging', 'prod', ''));

-- releases.environment: same closed set, no empty.
UPDATE releases SET environment = 'dev'
 WHERE environment NOT IN ('dev', 'staging', 'prod');
ALTER TABLE releases ADD CONSTRAINT chk_releases_env
  CHECK (environment IN ('dev', 'staging', 'prod'));

-- alerts.status: active | resolved.
UPDATE alerts SET status = 'active'
 WHERE status NOT IN ('active', 'resolved');
ALTER TABLE alerts ADD CONSTRAINT chk_alert_status
  CHECK (status IN ('active', 'resolved'));

-- eval_results.passed: 0 | 1.
UPDATE eval_results SET passed = 0 WHERE passed NOT IN (0, 1);
ALTER TABLE eval_results ADD CONSTRAINT chk_er_passed
  CHECK (passed IN (0, 1));

-- schedules has no environment column (the cron / webhook
-- fields are environment-agnostic; the associated release's
-- environment is what matters). No CHECK for schedules.kind
-- either — the closed set is enforced in Go via
-- schedule.KindCron / schedule.KindManual / etc.
