---
layout: page
title: Configuration
subtitle: Tune Promptsheon with environment variables.
---

# Configuration

Promptsheon reads its configuration from environment variables. Every variable is prefixed with `PROMPTSHEON_`. The single source of truth is [`packages/server/src/config/env.ts`](https://github.com/sachncs/promptsheon/blob/master/packages/server/src/config/env.ts).

The table below is grouped by subsystem.

## Server

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_PORT` | HTTP listen port | `8080` |
| `PROMPTSHEON_HOST` | Bind address | `127.0.0.1` |
| `PROMPTSHEON_DB_PATH` | SQLite file path | `promptsheon.db` |
| `PROMPTSHEON_CAS_PATH` | Content-addressed store directory | `.promptsheon` |
| `PROMPTSHEON_FRONTEND_PATH` | Built frontend directory served from `:8080` | `./frontend/.next` |
| `PROMPTSHEON_CORS_ORIGIN` | Allowed CORS origin | `http://localhost:3000` |
| `PROMPTSHEON_LOG_LEVEL` | Pino log level (`fatal`, `error`, `warn`, `info`, `debug`, `trace`) | `info` |
| `PROMPTSHEON_NODE_ENV` | `production`, `development`, or `test` | `development` |
| `PROMPTSHEON_FIPS_MODE` | Enforce FIPS-validated crypto for the audit chain | `false` |

## Authentication

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_AUTH` | Enable JWT Bearer + SVID auth | `false` |
| `PROMPTSHEON_JWT_SECRET` | Required when `PROMPTSHEON_AUTH=true` | `""` |
| `PROMPTSHEON_ALLOW_SYSTEM_ACTOR` | Allow `X-User-Id: api` bypass in dev/test | dev-only default `true` |

When `PROMPTSHEON_AUTH=true`, any request without a `Bearer` or `SVID` header is rejected with `401 UNAUTHORIZED`. The legacy `X-User-Id` fallback is honoured only when `PROMPTSHEON_AUTH=false`. See [Security]({{ '/security/' | relative_url }}) for the threat model.

## LLM providers

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_LLM_PROVIDER` | `openai`, `anthropic`, `bedrock`, or `custom` | `openai` |
| `PROMPTSHEON_LLM_MODEL` | Model id (for example, `gpt-4`, `claude-3-5-sonnet-20241022`) | `gpt-4` |
| `PROMPTSHEON_LLM_API_KEY_ENV` | Env var name holding the API key | `OPENAI_API_KEY` |
| `PROMPTSHEON_LLM_MAX_RETRIES` | Strands retry budget | `5` |
| `PROMPTSHEON_LLM_TIMEOUT_MS` | Per-request LLM timeout | `120000` |
| `OPENAI_API_KEY` | OpenAI provider key | — |
| `ANTHROPIC_API_KEY` | Anthropic provider key | — |
| `AWS_BEDROCK_REGION` | Bedrock region (for example, `us-east-1`) | `us-east-1` |

For a **Custom** OpenAI/Anthropic-compatible endpoint, set `PROMPTSHEON_LLM_PROVIDER=custom` and supply the base URL, the model name, and the API key during the onboarding wizard. The settings store persists them per-org.

## Self-evolution

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_SELF_EVOLVE_ENABLED` | Enable the self-evolution loop | `false` |
| `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC` | Minimum seconds between re-evolves | `900` |
| `PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT` | Cap concurrent evolutions per worker | `3` |

## Audit-chain replication

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_REPLICA_INTERVAL_MS` | Replicator poll interval | `5000` |
| `PROMPTSHEON_REPLICA_ONESHOT` | Replicator exits after a single batch | `false` |

## Observability

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_OTEL_ENDPOINT` | OpenTelemetry OTLP collector URL | `""` |

## Webhooks

| Variable | Purpose | Default |
|----------|---------|---------|
| `PROMPTSHEON_WEBHOOK_SECRET` | HMAC secret for incoming webhooks | dev-only fallback |

`PROMPTSHEON_WEBHOOK_SECRET` is required in production (`PROMPTSHEON_NODE_ENV=production`). The server refuses to boot without it. See [Architecture]({{ '/architecture/' | relative_url }}) for the webhook contract.

## Custom `.env`

Copy `.env.example` to `.env` and edit as needed. The repo's `.gitignore` blocks `.env` from being committed, so each developer or operator keeps their own overrides.

## Validate at boot

The server calls `validateConfig()` immediately after `loadConfig()`. A missing `PROMPTSHEON_JWT_SECRET` when `PROMPTSHEON_AUTH=true`, or a port outside `1..65535`, fails the boot with a clear error. See [`packages/server/src/config/validate.ts`](https://github.com/sachncs/promptsheon/blob/master/packages/server/src/config/validate.ts).
