# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive documentation suite (docs/)
- Dockerfile with multi-stage build
- GoReleaser configuration for automated releases
- Dependabot configuration for dependency updates
- Security scanning in CI (gosec, govulncheck)
- Security headers middleware (X-Content-Type-Options, X-Frame-Options, etc.)
- Request body size limiting middleware (MaxBytes)
- Readiness probe now checks database connectivity
- Shell tool sandboxing with command allowlist/blocklist
- CHANGELOG.md

### Changed
- **BREAKING**: Authentication is now enabled by default. Set `PROMPTSHEON_AUTH=false` to disable.
- **BREAKING**: API key query parameter (`?api_key=`) support removed for security. Use Authorization header instead.
- CORS middleware is now configurable (still defaults to `*` for backward compatibility).
- SDK client now has full doc comments on all exported methods.
- Audit chain error handling is now strict (errors are no longer silently ignored).

### Fixed
- 58 files reformatted with gofmt
- Audit chain integrity on DB read failure (was silently ignored)
- Eval run persistence error handling (was silently discarded)
- Health version string is now consistent

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
