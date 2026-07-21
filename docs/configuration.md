# Configuration

Promptsheon is configured entirely through environment variables. There are no config files. The `config` package is the single source of truth: every variable is read in `internal/config/config.go` and the defaults are documented below.

> **For rationale, see [Architecture](architecture.md), [Security](security.md), and the relevant ADRs.** The tables below are the operator's reference.

## Server

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_ADDR` | `:8080` | TCP address for the HTTP server (`host:port`). |
| `PROMPTSHEON_DB_PATH` | `promptsheon.db` | Path to the SQLite database file. |
| `PROMPTSHEON_AUTH` | `true` | Enable API key authentication. Set to `false` (or `0`, `no`) only for local development. |
| `PROMPTSHEON_BOOTSTRAP_TOKEN` | *(empty)* | When set, `POST /api/v1/setup` is reachable even with `PROMPTSHEON_AUTH=true`. The caller must send the same value in the `X-Bootstrap-Token` header; the daemon compares it with `subtle.ConstantTimeCompare`. Empty (the default) keeps the route unregistered when auth is on, so the only way to mint the first admin key is to start the daemon with auth off, mint, then restart with auth on. |
| `PROMPTSHEON_LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `PROMPTSHEON_LOG_FORMAT` | `json` | `json` (default) or `text`. |
| `PROMPTSHEON_CORS_ORIGINS` | *(empty / deny-all)* | Comma-separated list of allowed origins. Empty (the production default) denies every cross-origin request; set to `*` only for trusted local development. |
| `PROMPTSHEON_VAULT_KEY` | *(empty)* | 64-character hex (32 bytes) encryption key for provider API keys. The all-zero key is rejected. |
| `PROMPTSHEON_AUTHOR` | *(empty)* | Default `author` field for CLI commits. |
| `PROMPTSHEON_SERVER_READ_TIMEOUT` | `30` | `http.Server.ReadTimeout` in seconds. |
| `PROMPTSHEON_SERVER_WRITE_TIMEOUT` | `60` | `http.Server.WriteTimeout` in seconds. |
| `PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT` | `10` | `http.Server.ReadHeaderTimeout` in seconds (Slowloris defence). |
| `PROMPTSHEON_SERVER_IDLE_TIMEOUT` | `120` | `http.Server.IdleTimeout` in seconds. |
| `PROMPTSHEON_TELEMETRY` | *(empty)* | CLI commit telemetry blob (key=value pairs separated by `,`). CLI only. |

### Example

```bash
PROMPTSHEON_ADDR=:9090 \
PROMPTSHEON_DB_PATH=/data/promptsheon.db \
PROMPTSHEON_AUTH=true \
PROMPTSHEON_LOG_LEVEL=info \
PROMPTSHEON_VAULT_KEY=2c5b9a3e4d1f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b \
./promptsheond
```

## Server timeouts

These are not exposed as env vars today. The defaults are set in `internal/config/config.go`:

| Timeout | Default | Why |
|---|---|---|
| `ReadTimeout` | 30s | Bound the request read window. |
| `ReadHeaderTimeout` | 10s | Slowloris defence. |
| `WriteTimeout` | 60s | Bound the response write window. |
| `IdleTimeout` | 120s | Bound the keep-alive idle window. |

## LLM provider keys

These are read by the provider constructors. Two forms are accepted: the upstream provider's name (no `PROMPTSHEON_` prefix) and the Promptsheon-prefixed alias. Pick one and be consistent.

| Variable | Description |
|---|---|
| `OPENAI_API_KEY` (or `PROMPTSHEON_OPENAI_API_KEY`) | OpenAI API key (`sk-...`). |
| `OPENAI_BASE_URL` (or `PROMPTSHEON_OPENAI_BASE_URL`) | Override the OpenAI base URL. |
| `ANTHROPIC_API_KEY` (or `PROMPTSHEON_ANTHROPIC_API_KEY`) | Anthropic API key (`sk-ant-...`). |
| `ANTHROPIC_BASE_URL` (or `PROMPTSHEON_ANTHROPIC_BASE_URL`) | Override the Anthropic base URL. |
| `PROMPTSHEON_AZURE_API_KEY` | Azure OpenAI API key. (Documented for future use; no Azure provider is wired into the registry in v0.1.x.) |
| `PROMPTSHEON_AZURE_RESOURCE` | Azure OpenAI endpoint URL. |
| `PROMPTSHEON_AZURE_API_VERSION` | Azure OpenAI API version (e.g. `2024-02-15-preview`). |
| `PROMPTSHEON_AZURE_DEPLOYMENT` | Azure OpenAI deployment name. |
| `PROMPTSHEON_OLLAMA_BASE_URL` | Ollama server URL (default: `http://localhost:11434`). Read-only; no Ollama provider is wired into the registry in v0.1.x. |
| `PROMPTSHEON_NVIDIA_API_KEY` | NVIDIA NIM API key. (Documented for future use; no NVIDIA provider is wired into the registry in v0.1.x.) |
| `PROMPTSHEON_NVIDIA_BASE_URL` | Override the NVIDIA NIM base URL. |
| `PROMPTSHEON_LLM_PROVIDER` | Default provider name to use when a request does not specify one. |

See [LLM Providers](llm-providers.md) for per-provider setup. See [Algorithms — Vault](algorithms.md#vault-aes-256-gcm) for storing keys in the encrypted vault.

## LLM resilience

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_CIRCUIT_BREAKER_FAILURE_THRESHOLD` | `5` | Consecutive failures before the breaker opens. |
| `PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD` | `3` | Consecutive successes in half-open before the breaker closes. |
| `PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN` | `30` | Seconds the breaker stays open before half-open. |
| `PROMPTSHEON_LLM_FALLBACK` | *(empty)* | Comma-separated fallback provider list (e.g. `anthropic,openai`). Documented for future use; the per-call fallback chain is not wired into the production invocation path in v0.1.x. |

## Rollups (ClickHouse)

| Variable | Description |
|---|---|
| `PROMPTSHEON_CLICKHOUSE_DSN` | ClickHouse DSN. When set AND the binary is built with `-tags clickhouse`, the per-workspace rollup aggregator forwards summaries to ClickHouse. Without the build tag the env var surfaces a clear diagnostic at startup. |

See [Algorithms — Circuit breaker](algorithms.md#circuit-breaker) and [Algorithms — Fallback chain](algorithms.md#fallback-chain).

## OAuth

| Variable | Description |
|---|---|
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_ID` | Google OAuth client ID. |
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_SECRET` | Google OAuth client secret. |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_ID` | GitHub OAuth client ID. |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_SECRET` | GitHub OAuth client secret. |

## Rate limiting

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_RATE_LIMIT` | `100` | Tokens per interval. `0` disables rate limiting cleanly (no implicit 1M-token burst). |
| `PROMPTSHEON_RATE_LIMIT_RATE` | `100` | Alias for `PROMPTSHEON_RATE_LIMIT`. |
| `PROMPTSHEON_RATE_BURST` | `50` | Token-bucket burst capacity. Set explicitly when `RATE_LIMIT=0` to grant a specific large burst. |
| `PROMPTSHEON_RATE_LIMIT_BURST` | `50` | Alias for `PROMPTSHEON_RATE_BURST`. |
| `PROMPTSHEON_RATE_LIMIT_INTERVAL` | `60` | Interval in seconds. |

## Shell tool (workflow engine)

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_SHELL_ENABLED` | `false` | Enable the workflow `shell` tool. Must be `true` **and** the allowlist non-empty for the tool to be active. |
| `PROMPTSHEON_SHELL_ALLOWLIST` | *(empty)* | Comma-separated list of allowed base commands. |

The tool is **fail-closed**. An empty allowlist with the tool "enabled" is treated as disabled (the server logs a warning and forces the enabled flag to `false`). See [Security](security.md#shell-tool-policy).

## Webhooks

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE` | `false` | Allow webhook destinations in loopback / private ranges. **Local development only.** |

## Retention

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_TRACE_TTL_DAYS` | `30` | Days to keep trace spans. Minimum 30. |
| `PROMPTSHEON_SNAPSHOT_TTL_DAYS` | `30` | Days to keep output snapshots. |
| `PROMPTSHEON_AUDIT_TTL_DAYS` | `90` | Days to keep audit entries. |
| `PROMPTSHEON_RETENTION_CHECK_MINUTES` | `60` | How often the retention sweeper runs. |

See [Algorithms — Retention sweep](algorithms.md#retention-sweep).

## OpenTelemetry

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_OTEL_ENDPOINT` | *(empty)* | OTLP gRPC endpoint (e.g. `otel-collector:4317`). When empty, traces are kept in-process only. |
| `PROMPTSHEON_OTEL_INSECURE` | `false` | Use an insecure (non-TLS) connection to the collector. Accepts `true`, `1`, `yes`. |

See [Observability](observability.md).

## Database tuning

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_DB_BUSY_TIMEOUT` | `5000` | SQLite busy timeout in milliseconds. |
| `PROMPTSHEON_DB_CACHE_SIZE` | `-64000` | SQLite cache size hint (negative = KB). `-64000` is 64 MB. |

## Validation rules

The `config` package fails the server start with a clear error for these cases:

- `PROMPTSHEON_VAULT_KEY` is set but is not 64 hex characters.
- `PROMPTSHEON_VAULT_KEY` decodes to 32 zero bytes.
- `PROMPTSHEON_SHELL_ENABLED=true` with an empty `PROMPTSHEON_SHELL_ALLOWLIST` is coerced to disabled with a warning (fail-closed).
- Numeric env vars (timeouts, thresholds) must parse as positive integers.

## Precedence

Environment variables take precedence over compiled-in defaults. There is no `.env` file loader; if you want one, set the variables in your shell or in a wrapper script.

## Test-only variables

These variables are read by the test suite, not by the server. They are listed here for completeness; operators do not need to set them.

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_AUTH_TEST` | *(empty)* | When set to `true`, integration tests enable API key auth. See `internal/api/api_comprehensive_test.go`. |

## See also

- [Getting Started](getting-started.md) — first-run configuration
- [Security](security.md) — the threat model and the operator checklist
- [Deployment](deployment.md) — production deployment
- [Troubleshooting](troubleshooting.md) — when something goes wrong
