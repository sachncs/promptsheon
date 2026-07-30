# Promptsheon Documentation

**Version Control System for AI Agent Intelligence**

Welcome to the Promptsheon documentation. This is the master index. The full source of truth for the wire format is [`backend/spec/spec.yaml`](../../backend/spec/spec.yaml); this site is the human-readable counterpart.

## Audience map

| If you are a… | Start here |
|---|---|
| New user who wants to run the server | [Getting Started](../development/getting-started.md) |
| Operator who needs to configure the server | [Configuration](../operations/configuration.md) |
| Operator who needs to deploy the server | [Deployment](../operations/deployment.md) |
| Operator who needs to debug an issue | [Troubleshooting](../operations/troubleshooting.md) |
| User who wants to use the CLI | [CLI](../development/cli.md) |
| Developer integrating with the API | [API Reference](../reference/api-reference.md) and the [SDK](../reference/sdk.md) |
| Contributor | [Development](../development/development.md) and [Testing](../development/testing.md) |
| Reviewer / compliance | [Security](../security/security.md) |

## User

| Document | Description |
|---|---|
| [Getting Started](../development/getting-started.md) | First run, build, and basic usage. |
| [Configuration](../operations/configuration.md) | Every environment variable and its default. |
| [LLM Providers](../reference/llm-providers.md) | OpenAI and Anthropic provider wiring; how to add a new one. |
| [CLI](../development/cli.md) | The `promptsheon` client binary. |
| [SDK](../reference/sdk.md) | The Go, Python, and TypeScript client libraries. |
| [API Reference](../reference/api-reference.md) | Human summary of the REST API. The [OpenAPI spec](../../backend/spec/spec.yaml) is the source of truth. |
| [Workflows](../reference/workflows.md) | DAG-based multi-step agents. |
| [Harness engineering](../reference/harness.md) | Why the harness surface exists; the Capability / Version / Release / Eval stack. |
| [Evaluations](../reference/eval.md) | Datasets, preconditions, eval runs — the harness loop. |
| [Guardrails](../reference/guardrails.md) | Content policy enforcement. |
| [FAQ](../development/faq.md) | Frequently asked questions. |
| [Glossary](glossary.md) | Terminology reference. |

## Operator

| Document | Description |
|---|---|
| [Deployment](../operations/deployment.md) | Production build, systemd, Docker, nginx, monitoring. |
| [Security](../security/security.md) | Threat model, controls, vulnerability reporting. |
| [Observability](../operations/observability.md) | Logs, traces, metrics, audit, retention. |
| [Troubleshooting](../operations/troubleshooting.md) | The operator runbook. |
| [Configuration](../operations/configuration.md) | Every environment variable and its default. |

## Developer and contributor

| Document | Description |
|---|---|
| [Architecture](architecture.md) | System diagram, package layout, request lifecycle. |
| [Modules](modules.md) | One-line purpose for every Go package. |
| [Algorithms](algorithms.md) | BM25, retry, circuit breaker, fallback, cost, vault, audit chain, HMAC, workflow DAG execution, retention. |
| [Design Decisions](design-decisions.md) | The architectural rationale behind key choices. |
| [Development](../development/development.md) | Setup, layout, Make targets, OpenAPI generator, migrations. |
| [Testing](../development/testing.md) | Test layers, helpers, race detection, coverage. |
| [API Reference — Generator](../reference/api-reference.md#generator) | How `backend/spec/spec.yaml` is produced. |

## Quick links

- **OpenAPI spec**: [`backend/spec/spec.yaml`](../../backend/spec/spec.yaml)
- **Server health**: `GET /health`
- **Server readiness**: `GET /ready`
- **Prometheus metrics**: `GET /metrics`
- **Audit chain verify**: `GET /api/v1/audit/verify`
- **CLI help**: `./promptsheon help`
- **Server help**: `./promptsheond --help` (configuration is via env vars — see [Configuration](../operations/configuration.md))
- **Makefile targets**: `make help`

## Authoring guide

If you are adding a new doc:

- One sentence in the first paragraph stating what the doc is for.
- One H1 (`#`), then H2s (`##`). No H3-H6.
- Tables for parallel data, not bullet lists.
- Code blocks must have a language tag (` ```bash `, ` ```go `, ` ```json `, ` ```text `, etc.).
- Use the [Glossary](glossary.md) terms verbatim. No synonyms.
- Link with relative paths inside `docs/`. Use `../../backend/spec/spec.yaml` for the OpenAPI spec.
- End with a "See also" section if the doc is referenced from elsewhere.
- Add a row to this index, in the right audience group.

If you are changing a doc:

- Update the "Last reviewed" footer to the current date.
- Run `make fmt` and `make lint` if you have them. Docs do not have a linter step in CI yet; a `markdownlint` config is a future addition.
