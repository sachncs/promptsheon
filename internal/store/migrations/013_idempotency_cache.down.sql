-- Roll back 013_idempotency_cache.

DROP INDEX IF EXISTS idempotency_cache_expires_at_idx;
DROP TABLE IF EXISTS idempotency_cache;
