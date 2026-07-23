# Configuration

Promptsheon is configured via environment variables. There is
no config file format — operators export the variables they want
and the daemon reads them at boot.

The daemon refuses to start when a configuration is unsafe
(e.g. `PROMPTSHEON_AUTH=false` on a non-loopback bind) and
returns a clear error message naming the offending setting.

## Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_ADDR` | `:8080` | Listen address (`host:port` or `:port`). |
| `PROMPTSHEON_DB_PATH` | `promptsheon.db` | SQLite database file. |
| `PROMPTSHEON_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `PROMPTSHEON_AUTH` | `true` | Enable API key authentication. **Required `true` for non-loopback binds** — the daemon refuses to start otherwise. |
| `PROMPTSHEON_TLS_CERT_FILE` | (none) | Path to a PEM-encoded TLS certificate. Required for non-loopback binds. |
| `PROMPTSHEON_TLS_KEY_FILE` | (none) | Path to a PEM-encoded TLS private key. Required for non-loopback binds. |
| `PROMPTSHEON_BOOTSTRAP_TOKEN` | (none) | Optional gate for `POST /api/v1/setup` when `PROMPTSHEON_AUTH=true`. The caller must present the value via `X-Bootstrap-Token`. |
| `PROMPTSHEON_LEADER_ELECTION` | `false` | When `true`, enables SQLite-backed leader election across replicas. Single-replica deployments leave this off. |
| `PROMPTSHEON_RETENTION_CHECK_MINUTES` | `60` | Retention sweep interval. |
| `PROMPTSHEON_TRACE_TTL_DAYS` | `30` | Trace retention; minimum 30 days enforced. |
| `PROMPTSHEON_AUDIT_TTL_DAYS` | `90` | Audit chain retention. |

## LLM providers

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_LLM_PROVIDER` | (none) | The default provider name (`openai` or `anthropic`). |
| `PROMPTSHEON_OPENAI_API_KEY` | (none) | OpenAI API key. |
| `PROMPTSHEON_OPENAI_BASE_URL` | (none) | OpenAI base URL override (for proxies). |
| `PROMPTSHEON_ANTHROPIC_API_KEY` | (none) | Anthropic API key. |
| `PROMPTSHEON_ANTHROPIC_BASE_URL` | (none) | Anthropic base URL override. |

The daemon supports **OpenAI** and **Anthropic** in v0.1.x. Azure
OpenAI, Ollama, and NVIDIA NIM were removed in v0.1.0. To add a
new provider, register it on the LLM `Registry` in
`cmd/promptsheond/main.go` and write the SDK adapter under
`internal/llm/`.

## Vault

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_VAULT_KEY` | (none) | 32-byte hex master key for the AES-256-GCM vault. Override with a KMS-backed `KeyProvider` for production. |
| `PROMPTSHEON_CLICKHOUSE_DSN` | (none) | ClickHouse DSN (used only when the binary is built with `-tags clickhouse`). |

## OAuth

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_ID` | (none) | Google OAuth client ID. |
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_SECRET` | (none) | Google OAuth client secret. |
| `PROMPTSHEON_OAUTH_GOOGLE_REDIRECT_URL` | (none) | Google OAuth redirect URL. |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_ID` | (none) | GitHub OAuth client ID. |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_SECRET` | (none) | GitHub OAuth client secret. |
| `PROMPTSHEON_OAUTH_GITHUB_REDIRECT_URL` | (none) | GitHub OAuth redirect URL. |
| `PROMPTSHEON_OAUTH_AUTO_PROVISION` | `false` | When `true`, unknown OAuth users are auto-provisioned as `reader`. **Default `false`** — an admin must pre-create the user. |

## Observability

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_OTEL_ENDPOINT` | (none) | OTLP gRPC endpoint for trace export. When unset, traces go to a noop exporter. |
| `PROMPTSHEON_OTEL_SAMPLE_RATIO` | `1.0` | OTel trace sampling ratio (0.0–1.0). Production deployments with high request volume typically dial this to `0.1` or below. |
| `PROMPTSHEON_OTEL_INSECURE` | `false` | Use insecure gRPC connection to the OTel collector. |

## Approval

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_APPROVAL_POLICY` | `maker_checker` | `maker_checker` (creator cannot approve their own release; at least one other identity must) or `majority` (flat count-based). See [docs/release.md](release.md). |

## Plugins

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_PLUGINS_FILE` | (none) | Path to the plugin manifest. Each entry is one plugin binary; the supervisor launches, supervises, and restarts them. |
| `PROMPTSHEON_HARNESS_PRECONDITIONS` | `false` | When `true`, the precondition runner is enabled. Default is off so unconfigured deployments don't accidentally execute hooks. |

## Removed env vars

The following env vars were removed in recent versions and are
no longer read. Setting them is a no-op.

- `PROMPTSHEON_LOG_FORMAT` (json|text) — output is always JSON.
- `PROMPTSHEON_AUTHOR`, `PROMPTSHEON_TELEMETRY` — CLI-only,
  not read by the server.
- `PROMPTSHEON_LLM_FALLBACK` — the per-call fallback chain
  lives on the LLM `Registry`; configure multiple providers
  per model instead.
- `PROMPTSHEON_DB_BUSY_TIMEOUT`, `PROMPTSHEON_DB_CACHE_SIZE` —
  hardcoded at `?_pragma=busy_timeout(5000)` in `cmd/promptsheond/main.go`.
- `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE` — per-endpoint allowlist
  was removed (SEC-4). Webhooks only accept HTTPS to non-private IPs.
- `PROMPTSHEON_AZURE_*`, `PROMPTSHEON_OLLAMA_*`, `PROMPTSHEON_NVIDIA_*`
  — providers removed in v0.1.0.
- `PROMPTSHEON_AUTH_TEST` — no consumer.
- `PROMPTSHEON_METRICS_ADDR` — the metrics endpoint binds on
  the same address as the API; gate it via `PermAuditRead`.