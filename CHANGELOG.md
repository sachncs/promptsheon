# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Comprehensive documentation suite ([docs/](docs/)) with audience-grouped index, modules map, algorithms, ADRs, security model, observability guide, CLI/SDK references, development and testing handbooks, FAQ, and glossary.
- Architecture Decision Records under [docs/adr/](docs/adr/) (CAS, BM25, audit chain, AES-256-GCM vault, HMAC webhooks with SSRF allowlist, modernc.org/sqlite, slog-as-observability-foundation, topological workflow DAG, prompt-binding 0.3.0).
- Hash-chained audit log with `audit_chain_state` cache and paged verifier (migrations `006`, `020`, `021`).
- AES-256-GCM vault with all-zero key rejection (`internal/vault/vault.go`).
- HMAC-SHA256 webhook signing with SSRF allowlist policy (`internal/webhook/`).
- Per-call LLM circuit breaker, retry with typed-error classifier, fallback chain, and per-model cost calculation (`internal/llm/{retry,circuitbreaker,fallback,cost}.go`).
- BM25 full-text search index with thread-safe Manager (`internal/search/`).
- Background audit workers with `StopAuditWorkers` for graceful shutdown.
- Background retention sweeper for traces, snapshots, and audit entries (`internal/observability/retention.go`).
- Token-bucket rate limiter, per API key (`internal/ratelimit/`).
- Security headers middleware (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, CSP).
- Request body size limit middleware (`MaxBytesReader`, 10 MB).
- CORS middleware with explicit origin allowlist.
- OpenTelemetry export when `PROMPTSHEON_OTEL_ENDPOINT` is set.
- AST-based OpenAPI generator in `scripts/genopenapi/` with `make openapi` and `make openapi-check`.

### Changed

- **BREAKING**: Authentication is enabled by default. Set `PROMPTSHEON_AUTH=false` to disable (local development only).
- **BREAKING**: API key query parameter (`?api_key=`) support removed. Use the `Authorization: Bearer` header.
- **BREAKING**: Prompt-binding schema is at version 0.3.0 with explicit `binding` block (`provider`, `model`, `parameters`, `api_key_ref`).
- The `Authorization` header value is masked in structured log output.
- The audit chain `append` is serialised in-process with a mutex; the in-DB state row is read in a single SQL query.
- API key `last_used_at` updates are now fire-and-forget on a background goroutine.
- `ReadHeaderTimeout` defaults to 10 seconds (Slowloris defence).
- CORS middleware is configurable (defaults to `*` for ease of local use).

### Fixed

- The vault now rejects the all-zero encryption key at startup.
- The audit chain now hashes the entry's `details` JSON and the canonical RFC3339Nano timestamp, closing a tamper window.
- The OpenAPI generator no longer emits empty or stub `200 OK` responses — each route gets a real request schema.
- `gofmt` is enforced in CI; unformatted code fails the build.
- Coverage floor raised to 70%.
- 58 files reformatted with `gofmt`.
- Audit chain integrity on DB read failure is no longer silently ignored.
- Eval run persistence error handling is no longer silently discarded.
- The health endpoint version string is now consistent.

## [0.1.0] - 2024-01-01

### Added

- Initial release
- Content-addressable storage (CAS) with Merkle DAG
- Prompt management with versioning
- Agent workflow execution engine
- Evaluation engine with scoring and hallucination detection
- LLM provider abstraction (OpenAI, Anthropic, Azure, Ollama, NVIDIA)
- Guardrails and content policy enforcement
- Alerting and threshold monitoring
- Observability with tracing and metrics
- Webhook integrations with HMAC signing
- OAuth/SSO authentication
- Go client SDK
