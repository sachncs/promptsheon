-- 010 down: drop ws_state. The Hub.nextID counter goes back to
-- 1 on restart; clients in memory are dropped (this is the
-- normal shutdown path).

DROP TABLE ws_state;
