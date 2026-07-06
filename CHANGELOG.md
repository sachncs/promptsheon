# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `internal/llm/types.go`: `Usage` type moved from `internal/models/eval.go` to
  `internal/llm/` to break the circular dependency between models and llm
  packages. All `models.Usage` references across the codebase updated to
  `llm.Usage`.
- `internal/store/repo.go`: `CapabilityRepository` interface embedded into the
  main `Repository` interface.

### Added

- `internal/capability` package: new capability-centric domain model with 18
  types (Workspace, Project, Capability, CapabilityVersion, Prompt,
  ModelPolicy, ContextContract, KnowledgeSource, MemoryConfig, Guardrail,
  Tool, MCPServer, RuntimePolicy, EvaluationSuite, Execution, Observation,
  EvaluationResult, Recommendation, Deployment, Event). This is the foundation
  for the capability-centric architecture — the Capability is the root object
  and everything else defines, executes, observes, or improves it.
- `internal/store/capability_repo.go`: `CapabilityRepository` interface with
  20 methods for persisting Workspaces, Projects, Capabilities,
  CapabilityVersions, and Executions.
- `internal/store/sqlite_capabilities.go`: SQLite implementation of
  `CapabilityRepository`.
- `internal/store/migrations/022_capabilities.sql`: schema for workspaces,
  projects, capabilities, capability_versions, and executions tables with
  indexes.
- `internal/workflow/engine_version.go`: `ExecuteVersion` function — executes
  a capability version through the workflow engine.
- `internal/eval/runner_version.go`: `RunVersion` function — evaluates a
  capability version against its evaluation suite.
- `internal/guardrail/manager_version.go`: `CheckVersion` function — checks
  a capability version against its configured guardrails.
- `internal/optimizer/optimizer_version.go`: `AnalyzeVersion` function —
  analyzes a capability version and produces optimization recommendations.
- `internal/context/manager_version.go`: `AssembleFromContract` function —
  assembles context from a capability version's context contract.
- `internal/buildinfo` package: central source of build version, commit, and
  build-time strings. Used by `--version`, `/api/v1/version`, the
  Prometheus `Content-Type` version parameter, and the startup banner.
- `promptsheond --version` and `promptsheon --version` flags print the
  build info (version, commit, build time, OS/arch).
- `promptsheond --help` flag prints the supported flags and the
  `PROMPTSHEON_*` environment variables that operators need to set.
- `GET /api/v1/version` returns build info as JSON; the endpoint is
  intentionally unauthenticated so external probes can read it.
- First-run bootstrap: `POST /api/v1/setup` mints an admin user and
  admin API key when `PROMPTSHEON_AUTH=false` and the user table is
  empty. The endpoint is removed the moment any user record exists.
  **Security note:** the endpoint is unauthenticated and gates
  entirely on "no users yet". Set `PROMPTSHEON_AUTH=true` before
  exposing the server. See `docs/security.md` and the comment on
  `handleBootstrap` in `internal/api/handlers_auth.go`.
- `sdk.NewWithHTTP` for callers that need to inject a custom
  `http.Client` (retry middleware, metrics instrumentation, mTLS).
- `sdk/README.md` documents install, quick start, error handling,
  and the API coverage table.
- `CODEOWNERS` at the repository root so GitHub auto-routes reviews.
- `.editorconfig` for cross-editor whitespace consistency.

### Changed

- Module path is now `github.com/sachn-cs/promptsheon` so external Go
  projects can `go get` the SDK and the CLI.
- Error message field on `sdk.APIError` is now correctly tagged
  `json:"error"` (the field the server emits). The decoder tries the
  canonical shape, then the legacy `message` field, and finally
  falls back to the raw body so non-JSON error responses still
  surface useful text to the caller.
- `cmd/promptsheon` exits with status 2 (EX_USAGE) for argument
  errors instead of 1, and prints a hint to run `promptsheon help`.
- `Config.Port()` uses `net.SplitHostPort` so IPv6 addresses like
  `[::1]:8080` resolve correctly.
- Startup banner includes version, commit, addr, db path, auth
  state, and OTel endpoint so a single log line lets an operator
  verify the running configuration.
- `Code of Conduct` and `SECURITY.md` route reports to private email
  addresses (`conduct@sachn-cs.dev`, `security@sachn-cs.dev`) rather
  than the public issue tracker.

### Fixed

- Build: `internal/promptsheon` (the CAS engine) was missing every
  type, constant, and function that the tests and CLI depended on.
  The package is now implemented and the full test suite passes.
- Dockerfile: the second `COPY` previously wrote the CLI binary to
  the wrong destination (`/usr/local/bin/promptsheon-cli` was never
  referenced by anything in the image). The CLI is now at
  `/usr/local/bin/promptsheon` inside the container.
- `internal/ratelimit.Limiter.Stop()` could panic with `close of nil
  channel` because `NewLimiter` never initialised the `cleanupDone`
  channel. The bug was latent; the first-run bootstrap test exposed
  it on every clean shutdown.
- 16 files were failing `gofmt -l`; all are now formatted.
- 14 hard-coded version strings across the codebase (handler,
  metrics, CHANGELOG) replaced by a single source of truth.

## [0.0.5] - 2026-06-25

### Added

- Hash-chained audit log with `audit_chain_state` cache and paged
  verifier.
- AES-256-GCM vault with all-zero key rejection.
- HMAC-SHA256 webhook signing with SSRF allowlist policy.
- Per-call LLM circuit breaker, retry with typed-error classifier,
  fallback chain, and per-model cost calculation.
- BM25 full-text search index.
- Background audit workers with `StopAuditWorkers` for graceful
  shutdown.
- Background retention sweeper for traces, snapshots, and audit
  entries.
- Token-bucket rate limiter, per API key.
- Security headers middleware.
- Request body size limit middleware (10 MB).
- CORS middleware with explicit origin allowlist.
- OpenTelemetry export when `PROMPTSHEON_OTEL_ENDPOINT` is set.
- AST-based OpenAPI generator in `scripts/genopenapi/`.

### Changed

- Authentication is enabled by default; set `PROMPTSHEON_AUTH=false`
  to disable (local development only).
- API key query parameter (`?api_key=`) support removed. Use the
  `Authorization: Bearer` header.
- Prompt-binding schema is at version 0.3.0 with explicit `binding`
  block.
- The `Authorization` header value is masked in structured log output.

### Fixed

- The vault now rejects the all-zero encryption key at startup.
- The audit chain now hashes the entry's `details` JSON and the
  canonical RFC3339Nano timestamp.
- The OpenAPI generator no longer emits empty `200 OK` responses.
- `gofmt` is enforced in CI.
- Coverage floor raised to 70%.

## [0.0.4] - 2026-06-25

Internal hardening: gosec fixes, lint cleanups, audit chain
rework, rate-limit middleware.

## [0.0.3] - 2026-06-19

Initial public release of the workflow engine, eval runner, and
agent guardrail configuration endpoints.

## [0.0.2] - 2026-06-18

Internal API surface and auth subsystem; not published externally.

## [0.0.1] - 2026-06-18

First tagged build. Internal alpha.
