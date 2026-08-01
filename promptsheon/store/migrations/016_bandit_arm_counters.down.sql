-- Roll back 016_bandit_arm_counters.
DROP INDEX IF EXISTS bandit_arm_counters_updated_at_idx;
DROP TABLE IF EXISTS bandit_arm_counters;