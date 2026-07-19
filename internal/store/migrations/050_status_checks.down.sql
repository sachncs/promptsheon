-- Migration 050 down: drop the CHECK constraints. The data is
-- unchanged; the constraints were defensive only.

ALTER TABLE executions   DROP CONSTRAINT chk_executions_env;
ALTER TABLE releases     DROP CONSTRAINT chk_releases_env;
ALTER TABLE alerts       DROP CONSTRAINT chk_alert_status;
ALTER TABLE eval_results DROP CONSTRAINT chk_er_passed;
