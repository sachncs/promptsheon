# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- README: content upgrade with architecture diagram, configuration table, and
  table of contents; added Go version badge
  ([`2aba5bb`](https://github.com/sachncs/promptsheon/commit/2aba5bb), 2026-07-07)
- CI: bump `golangci/golangci-lint-action` from v6 to v7 to use Node 24
  (Node 20 is being deprecated on GitHub Actions runners)
  ([`2aba5bb`](https://github.com/sachncs/promptsheon/commit/2aba5bb), 2026-07-07)

### Added

- Capability-centric domain model with 18 types (Workspace, Project, Capability,
  CapabilityVersion, Prompt, ModelPolicy, ContextContract, KnowledgeSource,
  MemoryConfig, Guardrail, Tool, MCPServer, RuntimePolicy, EvaluationSuite,
  Execution, Observation, EvaluationResult, Recommendation, Deployment, Event)
  — foundation for the capability-centric architecture
  ([`58a21f0`](https://github.com/sachncs/promptsheon/commit/58a21f0), 2026-07-06)
- `CapabilityRepository` interface with SQLite implementation and version-based
  internal functions
  ([`60f0579`](https://github.com/sachncs/promptsheon/commit/60f0579), 2026-07-06)
- Embed `CapabilityRepository` into the main `Repository` interface; move `Usage`
  type to `llm` package to break circular dependency
  ([`b85097e`](https://github.com/sachncs/promptsheon/commit/b85097e), 2026-07-06)
- Cutover to capability-centric architecture (new handlers, store, migrations)
  ([`d5b0ee3`](https://github.com/sachncs/promptsheon/commit/d5b0ee3), 2026-07-06)

### Changed

- Module path from `github.com/sachn-cs/promptsheon` to `github.com/sachncs/promptsheon`
  to match the canonical remote
  ([`34342ca`](https://github.com/sachncs/promptsheon/commit/34342ca), 2026-06-25)

### Fixed

- CAS engine package (`internal/promptsheon`) implemented — previously all types,
  constants, and functions were missing despite being depended on by tests and CLI
  ([`3b60646`](https://github.com/sachncs/promptsheon/commit/3b60646), 2026-06-25)
- Dockerfile: CLI binary destination corrected from
  `/usr/local/bin/promptsheon-cli` to `/usr/local/bin/promptsheon`
  ([`ec2f093`](https://github.com/sachncs/promptsheon/commit/ec2f093), 2026-06-25)
- Centralised version string into `internal/buildinfo` package; added
  `/api/v1/version` endpoint
  ([`f441f13`](https://github.com/sachncs/promptsheon/commit/f441f13), 2026-06-25)
- `--version` and `--help` flags for both `promptsheond` and `promptsheon`
  binaries
  ([`1993f8b`](https://github.com/sachncs/promptsheon/commit/1993f8b), 2026-06-25)
- CLI exits with status 2 (EX_USAGE) for argument errors instead of 1
  ([`1fa9dea`](https://github.com/sachncs/promptsheon/commit/1fa9dea), 2026-06-25)
- First-run admin bootstrap endpoint (`POST /api/v1/setup`); ratelimit
  cleanup channel no longer panics on stop
  ([`ec6f6db`](https://github.com/sachncs/promptsheon/commit/ec6f6db), 2026-06-26)
- SDK: surface API error unmarshal failure; add tests and README
  ([`67609d7`](https://github.com/sachncs/promptsheon/commit/67609d7), 2026-06-26)
- `Config.Port()` uses `net.SplitHostPort` so IPv6 addresses resolve correctly
  ([`a103b95`](https://github.com/sachncs/promptsheon/commit/a103b95), 2026-06-26)
- Docker HEALTHCHECK respects `PROMPTSHEON_ADDR`
  ([`0224173`](https://github.com/sachncs/promptsheon/commit/0224173), 2026-06-26)
- Code formatting: `gofmt -s` applied across the repo
  ([`264209a`](https://github.com/sachncs/promptsheon/commit/264209a),
  [`0269bab`](https://github.com/sachncs/promptsheon/commit/0269bab), 2026-06-25–26)

### Added (test coverage)

- Tests for previously-untested packages
  ([`b9ccf71`](https://github.com/sachncs/promptsheon/commit/b9ccf71), 2026-06-26)
- Workflow state, webhook sink, server bootstrap tests
  ([`c88caaa`](https://github.com/sachncs/promptsheon/commit/c88caaa), 2026-06-26)
- A/B testing handlers, LLM providers, alerting monitoring tests
  ([`f2a72a4`](https://github.com/sachncs/promptsheon/commit/f2a72a4), 2026-06-26)
- Workflow YAML validation and round-trip tests
  ([`f860922`](https://github.com/sachncs/promptsheon/commit/f860922), 2026-06-26)
- Anthropic, Ollama, NVIDIA provider tests
  ([`0be808c`](https://github.com/sachncs/promptsheon/commit/0be808c), 2026-06-26)
- Metrics middleware tests
  ([`6bea48a`](https://github.com/sachncs/promptsheon/commit/6bea48a), 2026-06-26)
- Models package tests for JSON round-trip and state machine
  ([`caf007c`](https://github.com/sachncs/promptsheon/commit/caf007c), 2026-06-26)
- Webhook mutex-protected shared counters in race-mode tests
  ([`40e683d`](https://github.com/sachncs/promptsheon/commit/40e683d), 2026-06-26)

### Changed

- Coverage floor: lowered from 70% to 60% to reflect realistic bar for entry
  points that are not unit-testable
  ([`24041c3`](https://github.com/sachncs/promptsheon/commit/24041c3), 2026-06-26)
- Governance: add `CODEOWNERS`, `.editorconfig`, route CoC contact to email
  ([`56cb13d`](https://github.com/sachncs/promptsheon/commit/56cb13d), 2026-06-26)

### Dependencies

- Bump `securego/gosec` from 2.22.10 to 2.27.1
  ([`862582b`](https://github.com/sachncs/promptsheon/commit/862582b), 2026-06-29)
- Bump `google.golang.org/grpc` from 1.81.1 to 1.82.0
  ([`641863e`](https://github.com/sachncs/promptsheon/commit/641863e), 2026-07-06)

## [0.0.5] - 2026-06-25

### Added

- Hash-chained audit log with `audit_chain_state` cache and paged verifier
- AES-256-GCM vault with all-zero key rejection
- HMAC-SHA256 webhook signing with SSRF allowlist policy
- Per-call LLM circuit breaker, retry with typed-error classifier,
  fallback chain, and per-model cost calculation
- BM25 full-text search index
- Background audit workers with `StopAuditWorkers` for graceful shutdown
- Background retention sweeper for traces, snapshots, and audit entries
- Token-bucket rate limiter, per API key
- Security headers middleware
- Request body size limit middleware (10 MB)
- CORS middleware with explicit origin allowlist
- OpenTelemetry export when `PROMPTSHEON_OTEL_ENDPOINT` is set
- AST-based OpenAPI generator in `scripts/genopenapi/`

### Changed

- Authentication is enabled by default; set `PROMPTSHEON_AUTH=false` to disable
- API key query parameter (`?api_key=`) support removed; use `Authorization: Bearer`
- Prompt-binding schema is at version 0.3.0 with explicit `binding` block
- The `Authorization` header value is masked in structured log output

### Fixed

- Vault rejects the all-zero encryption key at startup
- Audit chain hashes the entry's `details` JSON and canonical RFC3339Nano timestamp
- OpenAPI generator no longer emits empty `200 OK` responses
- `gofmt` is enforced in CI
- Coverage floor raised to 70%

## [0.0.4] - 2026-06-25

### Added

- Code documentation annotations for intentional test panics
  ([`2609858`](https://github.com/sachncs/promptsheon/commit/2609858))

### Fixed (security)

- Refuse admin role in no-auth mode
  ([`59c5899`](https://github.com/sachncs/promptsheon/commit/59c5899))
- Set `ReadHeaderTimeout` to prevent Slowloris attacks
  ([`a7ec87f`](https://github.com/sachncs/promptsheon/commit/a7ec87f))
- OAuth callback no longer hides upstream error text
  ([`5634d61`](https://github.com/sachncs/promptsheon/commit/5634d61))

### Fixed (correctness)

- `handleDeletePrompt` pre-checks existence and skips audit on miss
  ([`d909aa7`](https://github.com/sachncs/promptsheon/commit/d909aa7))
- `handleRunPrompt` locks provider/model to prompt binding
  ([`ad490c9`](https://github.com/sachncs/promptsheon/commit/ad490c9))
- `handleListPromptVersions` returns prompt-scoped history, not global log
  ([`2813818`](https://github.com/sachncs/promptsheon/commit/2813818))
- `handleFindSimilarPrompts` validates threshold range
  ([`a9695a5`](https://github.com/sachncs/promptsheon/commit/a9695a5))
- `handleRestorePrompt` validates `cas_hash` and refuses non-blob CAS objects
  ([`fcb492b`](https://github.com/sachncs/promptsheon/commit/fcb492b))
- Wire guardrail + context manager into workflow engine
  ([`71a437e`](https://github.com/sachncs/promptsheon/commit/71a437e))
- `UpdateAPIKeyLastUsed` moved to background worker
  ([`4566388`](https://github.com/sachncs/promptsheon/commit/4566388))

### Fixed (reliability)

- `StopAuditWorkers` drains the queue, no more "database is closed" errors
  ([`30ed207`](https://github.com/sachncs/promptsheon/commit/30ed207))
- Replace hash-based embedding stub with real BM25 ranking
  ([`b97ce5d`](https://github.com/sachncs/promptsheon/commit/b97ce5d))
- Shell tool policy uses atomic types, no data race
  ([`f26af24`](https://github.com/sachncs/promptsheon/commit/f26af24))
- Persist workflow steps in topological order
  ([`b79517b`](https://github.com/sachncs/promptsheon/commit/b79517b))
- Server-owned persistent in-memory search index
  ([`c37f4f6`](https://github.com/sachncs/promptsheon/commit/c37f4f6))
- OpenAI.Stream returns a `streamCloser` that documents the close contract
  ([`7051fbf`](https://github.com/sachncs/promptsheon/commit/7051fbf))
- `mustMarshal` returns error instead of silently returning `{}`
  ([`e7ee76c`](https://github.com/sachncs/promptsheon/commit/e7ee76c))

### Fixed (observability)

- Add index on `traces.started_at` for range queries
  ([`50dd8bb`](https://github.com/sachncs/promptsheon/commit/50dd8bb))
- Wire `PROMPTSHEON_OTEL_*` env vars into the global tracer provider
  ([`414c5bf`](https://github.com/sachncs/promptsheon/commit/414c5bf))

### Fixed (security hardening)

- Vault rejects all-zero key (trivially decryptable)
  ([`e7b5b32`](https://github.com/sachncs/promptsheon/commit/e7b5b32))
- NVIDIA Nemotron `enable_thinking` is opt-in via provider config
  ([`106e02f`](https://github.com/sachncs/promptsheon/commit/106e02f))
- Audit hash uses canonical RFC3339Nano UTC string
  ([`0f9665d`](https://github.com/sachncs/promptsheon/commit/0f9665d))
- Audit chain uses dedicated state table + in-process mutex
  ([`784ed66`](https://github.com/sachncs/promptsheon/commit/784ed66))
- Audit queue backpressure waits briefly before dropping
  ([`ed469c8`](https://github.com/sachncs/promptsheon/commit/ed469c8))
- `VerifyAuditChain` paginates by rowid to bound per-call latency
  ([`8a25252`](https://github.com/sachncs/promptsheon/commit/8a25252))

### Fixed (code quality)

- Add 59 missing OpenAPI paths; align spec version to 0.3.0
  ([`5dfaef9`](https://github.com/sachncs/promptsheon/commit/5dfaef9))
- Go AST generator fills all 59 OpenAPI stubs
  ([`ada3987`](https://github.com/sachncs/promptsheon/commit/ada3987))
- Raise coverage floor from 50% to 70%
  ([`8b589e6`](https://github.com/sachncs/promptsheon/commit/8b589e6))
- `authEnabled` test global replaced with `PROMPTSHEON_AUTH_TEST` env
  ([`d62421b`](https://github.com/sachncs/promptsheon/commit/d62421b))
- CanonicalHash/ObjectHash return error instead of panicking
  ([`e8e32f5`](https://github.com/sachncs/promptsheon/commit/e8e32f5))
- Warn on invalid numeric env vars instead of silent fallback
  ([`a145cbe`](https://github.com/sachncs/promptsheon/commit/a145cbe))
- `substituteVariables` helper; prompt built once in run/stream
  ([`360599b`](https://github.com/sachncs/promptsheon/commit/360599b))
- Assert `http.Flusher` before writing SSE start event
  ([`aba520f`](https://github.com/sachncs/promptsheon/commit/aba520f))
- Install wget in Dockerfile; fix SECURITY.md contact info
  ([`5037ee8`](https://github.com/sachncs/promptsheon/commit/5037ee8))
- CI fails the build on gofmt drift
  ([`8740fb4`](https://github.com/sachncs/promptsheon/commit/8740fb4))
- Retry uses `ErrTransient`/`ErrPermanent` typed errors
  ([`2ef26d7`](https://github.com/sachncs/promptsheon/commit/2ef26d7))
- Drop unused loop index in `ParseYAML`
  ([`ad7dc26`](https://github.com/sachncs/promptsheon/commit/ad7dc26))
- `oauthStates` is per-Server, not package-level
  ([`2fcddfa`](https://github.com/sachncs/promptsheon/commit/2fcddfa))
- `gofmt` whitespace cleanup across files touched by fixes
  ([`36925e9`](https://github.com/sachncs/promptsheon/commit/36925e9),
  [`c32402f`](https://github.com/sachncs/promptsheon/commit/c32402f),
  [`80f5bbc`](https://github.com/sachncs/promptsheon/commit/80f5bbc))

### Dependencies

- Bump `modernc.org/sqlite` from 1.52.0 to 1.53.0
  ([`18118fd`](https://github.com/sachncs/promptsheon/commit/18118fd), 2026-06-22)
- Bump `actions/upload-artifact` from 4 to 7
  ([`6b0e034`](https://github.com/sachncs/promptsheon/commit/6b0e034), 2026-06-22)
- Bump `actions/checkout` from 4 to 7
  ([`ecaba2d`](https://github.com/sachncs/promptsheon/commit/ecaba2d), 2026-06-22)

## [0.0.3] - 2026-06-19

Version bump with no documented changes between 0.0.2 and 0.0.3 in git history.
([`aa0884b`](https://github.com/sachncs/promptsheon/commit/aa0884b))

## [0.0.2] - 2026-06-18

Version bump with no documented changes between 0.0.1 and 0.0.2 in git history.
([`3ff607b`](https://github.com/sachncs/promptsheon/commit/3ff607b))

## [0.0.1] - 2026-06-18

Initial release — first tagged build.
([`a1e289f`](https://github.com/sachncs/promptsheon/commit/a1e289f))

[Unreleased]: https://github.com/sachncs/promptsheon/compare/v0.0.5...HEAD
[0.0.5]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.5
[0.0.4]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.4
[0.0.3]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.3
[0.0.2]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.2
[0.0.1]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.1
