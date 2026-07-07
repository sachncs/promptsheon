# Troubleshooting

This page consolidates the operator-facing runbook. It supersedes the old root `TROUBLESHOOTING.md` (which is now a 3-line redirect).

If your problem is not listed here, open a [GitHub Issue](https://github.com/sachncs/promptsheon/issues) with the output of `make coverage-raw` and the relevant log lines.

## Server

### Server won't start

**Port already in use.**

```bash
lsof -i :8080
kill -9 <PID>
```

**Database locked.** SQLite uses a `*-shm` and `*-wal` file. A crashed server can leave them behind.

```bash
rm -f data/promptsheon.db-shm data/promptsheon.db-wal
./promptsheond
```

**Vault key is all zeros.** The vault refuses to start with an all-zero key. This is intentional — see ADR [0004](adr/0004-aes-256-gcm-vault.md). Generate a real key:

```bash
openssl rand -hex 32
# paste into PROMPTSHEON_VAULT_KEY
```

### `ReadHeaderTimeout` is set but you still see Slowloris errors

The default is 10 seconds. If you are behind a slow proxy, raise it:

```bash
# not exposed as an env var; edit the default in internal/config/config.go
# or use the deploy-time mechanism
```

### High memory usage

- Lower the trace TTL. The default is 30 days; a busy server can accumulate a lot of in-memory spans.
- Check for runaway goroutines: `pprof` on the `/debug/pprof/` endpoint.
- The `internal/observability/retention.go` sweeper runs every hour. If it cannot keep up, raise `PROMPTSHEON_RETENTION_CHECK_MINUTES` to a smaller value (more frequent sweeps).

## Authentication

### `401 Unauthorized`

- The header must be `Authorization: Bearer <key>`. The query-string variant (`?api_key=`) is disabled.
- The key must start with `ps_`.
- The key is checked against the `api_keys` table. If the key is not in the table, you get `401`.
- The `last_used_at` update is now fire-and-forget; it does not block the request.

### `403 Forbidden`

- The key is valid but does not have the role required for the operation. Roles: `admin`, `editor`, `viewer`.
- See `internal/auth/` for the role matrix.

### Auth disabled in development

```bash
export PROMPTSHEON_AUTH=false   # or 0, or no
```

Do **not** do this in production. The server logs a warning at startup when auth is disabled.

## LLM providers

### `circuit breaker is open`

The provider is in a temporary failure state. The breaker auto-recovers after `PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN` seconds (default 30). Wait, or change the fallback chain:

```bash
PROMPTSHEON_LLM_FALLBACK=anthropic,ollama
```

See ADR and algorithms for details.

### `model not found`

The model name is wrong for the provider. Common mistakes:

- `gpt-4-32k` was retired by OpenAI. Use `gpt-4-turbo`.
- Azure OpenAI uses the **deployment name**, not the model name, as the `model` field.

### `rate limit exceeded`

The provider returned a 429. The retry classifier treats 429 as transient by default; the call will be retried with backoff. If the rate is sustained, the circuit breaker will open.

### `ollama: connection refused`

```bash
# Is Ollama running?
curl http://localhost:11434
# Default URL: http://localhost:11434
# Override: PROMPTSHEON_OLLAMA_BASE_URL=http://my-host:11434
```

## Webhooks

### Delivery failed with `webhook destination is private`

The SSRF policy refused the destination. By default, loopback, link-local, and private ranges are blocked. See ADR [0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md).

```bash
# Only for local development:
PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE=true
```

The server logs this flag at startup. The flag is loud for a reason.

### Receiver says `signature mismatch`

The receiver must compute `HMAC-SHA256(secret, body)` over the **raw request body** — not the parsed JSON, not the pretty-printed JSON. The string comparison must be constant-time (`hmac.Equal` in Go).

## Audit chain

### `GET /api/v1/audit/verify` returns `{"ok": false, "reason": "..."}`

- The reason tells you which entry is wrong.
- Common causes:
  - A migration was applied to a database that was already running, and the canonical timestamp column was not backfilled. Migration `020_audit_canonical_ts.sql` backfills it; check that it ran.
  - Someone edited the database directly. Recover from a backup; the chain is linear, so a single corrupt row invalidates everything after it.
  - Clock skew between servers. The canonical timestamp is `RFC3339Nano` in UTC; a node running a different timezone will compute a different hash. Use NTP.

## Database

### Corrupt SQLite

```bash
# Stop the server, back up, then rebuild
cp promptsheon.db promptsheon.db.backup
sqlite3 promptsheon.db ".dump" | sqlite3 promptsheon-new.db
mv promptsheon-new.db promptsheon.db
./promptsheond
```

The audit chain survives a `.dump`/re-`.import` because the `entry_hash` and `previous_hash` columns are preserved.

### Permission denied

```bash
chmod 755 data/
chmod 640 data/promptsheon.db
```

The server runs as the `promptsheon` user. The database must be readable and writable by that user only.

### Migration failed mid-way

The migration runner in `internal/store/sqlite.go` records applied migrations in `schema_migrations`. Re-running the server re-applies any unapplied migration. If a migration is half-applied, the next start will fail with a clear error. Fix the underlying issue (often a missing index) and re-run.

## Performance

### Latency p99 above 1s

- Check `rate(llm_call_duration_seconds_sum[5m]) / rate(llm_call_duration_seconds_count[5m])` — is it the LLM or the server?
- If it is the server, check `http_request_duration_seconds` by `path`. The handler that dominates is the candidate.
- Open `pprof`:

```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

### Disk usage growing

- Raise the retention TTLs.
- Compact the audit chain: there is no compaction step, but you can archive old entries to cold storage and drop the rows.
- Use `sqlite3 promptsheon.db "VACUUM;"` after a large delete to reclaim space.

### SQLite busy

```bash
export PROMPTSHEON_DB_BUSY_TIMEOUT=5000    # milliseconds
export PROMPTSHEON_DB_CACHE_SIZE=-64000     # 64 MB
```

The busy timeout is the more important one. The cache size hint is a performance hint, not a guarantee.

## OpenAPI generator drift

If CI fails with `api/openapi.yaml is out of date`:

```bash
make openapi
git diff api/openapi.yaml
git add api/openapi.yaml
git commit
```

The generator is deterministic; running it twice on the same code produces the same output. See [Development — OpenAPI generator](development.md#openapi-generator).

## Logs

### Where are the logs?

- The structured log goes to **stderr** in JSON.
- The database has its own log table (`logs`) for application-level events; expose it via `GET /api/v1/logs/search`.

### Pretty-printing

```bash
./promptsheond 2>&1 | jq .
```

### Log level

```bash
export PROMPTSHEON_LOG_LEVEL=debug
```

Levels: `debug`, `info`, `warn`, `error`. The default is `info`.

## Getting help

- GitHub Issues: <https://github.com/sachncs/promptsheon/issues>
- GitHub Security Advisories: <https://github.com/sachncs/promptsheon/security/advisories/new>
- The [FAQ](faq.md) and the [Glossary](glossary.md) for terminology questions.
