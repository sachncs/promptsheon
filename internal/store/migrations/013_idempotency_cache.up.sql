-- 013: idempotency_cache (cross-replica Idempotency-Key replay).
--
-- API-IDEMP-1: the previous in-memory cache was per-replica,
-- so a POST retried via a different daemon would double-execute.
-- This table backs the IdempotencyStore interface in
-- internal/api/idempotency.go so every replica sees the same
-- replay window.
--
-- expires_at is indexed so the periodic cleanup (or a future
-- janitor goroutine) can prune expired rows in O(rows-scanned)
-- without scanning the whole table. The body column stores the
-- response bytes verbatim so the replay can re-emit them.

CREATE TABLE IF NOT EXISTS idempotency_cache (
    key         TEXT    PRIMARY KEY,
    expires_at  DATETIME NOT NULL,
    status_code INTEGER NOT NULL,
    headers     TEXT    NOT NULL,
    body        BLOB    NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idempotency_cache_expires_at_idx
    ON idempotency_cache(expires_at);
