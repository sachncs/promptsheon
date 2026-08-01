-- 019_self_evolve.down.sql
-- Reverses the self-evolution schema. The capabilities columns and
-- the self_evolve_state table are dropped. The state table is the
-- only source of non-derived data, so dropping it loses cooldown
-- history — operators must opt into this migration with
-- PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true.

DROP INDEX IF EXISTS idx_self_evolve_state_attempt;
DROP TABLE IF EXISTS self_evolve_state;

ALTER TABLE capabilities DROP COLUMN self_evolve_enabled;
ALTER TABLE capabilities DROP COLUMN self_evolve_min_score;
ALTER TABLE capabilities DROP COLUMN self_evolve_max_revisions;
ALTER TABLE capabilities DROP COLUMN self_evolve_cooldown_sec;
ALTER TABLE capabilities DROP COLUMN self_evolve_target_env;
ALTER TABLE capabilities DROP COLUMN self_evolve_dataset_id;
