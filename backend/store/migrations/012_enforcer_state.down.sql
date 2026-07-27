-- 012 down: drop enforcer_state. The enforcer falls back to
-- in-memory defaults after a restart.

DROP TABLE enforcer_state;
