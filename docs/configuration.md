# Configuration

Promptsheon is configured entirely through environment variables. There are no config files.

## Server Configuration

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_ADDR` | `:8080` | TCP address for the HTTP server (`host:port`) |
| `PROMPTSHEON_DB_PATH` | `promptsheon.db` | Path to the SQLite database file |
| `PROMPTSHEON_AUTH` | `false` | Enable API key authentication (`true`/`false`) |
| `PROMPTSHEON_LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `PROMPTSHEON_VAULT_KEY` | *(empty)* | Encryption key for provider API key storage |

### Example

```bash
PROMPTSHEON_ADDR=:9090 \
PROMPTSHEON_DB_PATH=/data/promptsheon.db \
PROMPTSHEON_AUTH=true \
PROMPTSHEON_LOG_LEVEL=debug \
PROMPTSHEON_VAULT_KEY=my-secret-encryption-key \
./promptsheond
```

## LLM Provider Keys

| Variable | Description |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key (sk-...) |
| `ANTHROPIC_API_KEY` | Anthropic API key (sk-ant-...) |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API key |
| `AZURE_OPENAI_ENDPOINT` | Azure OpenAI endpoint URL |
| `OLLAMA_BASE_URL` | Ollama server URL (default: http://localhost:11434) |
| `NVIDIA_API_KEY` | NVIDIA NIM API key |

See [LLM Providers](llm-providers.md) for detailed setup per provider.

## OAuth Configuration

| Variable | Description |
|---|---|
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `PROMPTSHEON_OAUTH_GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_ID` | GitHub OAuth client ID |
| `PROMPTSHEON_OAUTH_GITHUB_CLIENT_SECRET` | GitHub OAuth client secret |

## Rate Limiting

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_RATE_LIMIT_RPS` | `100` | Requests per second per IP |
| `PROMPTSHEON_RATE_LIMIT_BURST` | `200` | Burst capacity |

## Retention Policy

| Variable | Default | Description |
|---|---|---|
| `PROMPTSHEON_RETENTION_SPANS_DAYS` | `30` | Days to keep trace spans |
| `PROMPTSHEON_RETENTION_AUDIT_DAYS` | `90` | Days to keep audit entries |
| `PROMPTSHEON_RETENTION_SNAPSHOTS_DAYS` | `30` | Days to keep output snapshots |

## Server Timeouts

These are hardcoded in `cmd/promptsheond/main.go` but can be adjusted by modifying the source:

| Timeout | Value | Description |
|---|---|---|
| `ReadTimeout` | 30s | Maximum time to read the full request |
| `WriteTimeout` | 60s | Maximum time to write the response |
| `IdleTimeout` | 120s | Maximum time for keep-alive idle connections |
