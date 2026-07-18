# Promptsheon Documentation

**Version Control System for AI Agent Intelligence**

Welcome to the Promptsheon documentation. This is the master index. The full source of truth for the wire format is [`api/openapi.yaml`](../api/openapi.yaml); this site is the human-readable counterpart.

## Audience map

| If you are a… | Start here |
|---|---|
| New user who wants to run the server | [Getting Started](getting-started.md) |
| Operator who needs to configure the server | [Configuration](configuration.md) |
| Operator who needs to deploy the server | [Deployment](deployment.md) |
| Operator who needs to debug an issue | [Troubleshooting](troubleshooting.md) |
| User who wants to use the CLI | [CLI](cli.md) |
| Developer integrating with the API | [API Reference](api-reference.md) and the [SDK](sdk.md) |
| Contributor | [Development](development.md) and [Testing](testing.md) |
| Reviewer / compliance | [Security](security.md) and the [ADRs](adr/README.md) |

## User

| Document | Description |
|---|---|
| [Getting Started](getting-started.md) | First run, build, and basic usage. |
| [Configuration](configuration.md) | Every environment variable and its default. |
| [LLM Providers](llm-providers.md) | OpenAI, Anthropic, Azure OpenAI, Ollama, NVIDIA NIM. |
| [CLI](cli.md) | The `promptsheon` client binary. |
| [SDK](sdk.md) | The Go client library. |
| [API Reference](api-reference.md) | Human summary of the REST API. The [OpenAPI spec](../api/openapi.yaml) is the source of truth. |
| [Workflows](workflows.md) | DAG-based multi-step agents. |
| [Harness engineering](harness.md) | Why the harness surface exists; the Capability / Version / Release / Eval stack. |
| [Evaluations (v0.1.0)](eval.md) | Datasets, preconditions, eval runs — the harness loop. |
| [Guardrails](guardrails.md) | Content policy enforcement. |
| [FAQ](faq.md) | Frequently asked questions. |
| [Glossary](glossary.md) | Terminology reference. |

## Operator

| Document | Description |
|---|---|
| [Deployment](deployment.md) | Production build, systemd, Docker, nginx, monitoring. |
| [Security](security.md) | Threat model, controls, vulnerability reporting. |
| [Observability](observability.md) | Logs, traces, metrics, audit, retention. |
| [Troubleshooting](troubleshooting.md) | The operator runbook. |
| [Configuration](configuration.md) | Every environment variable and its default. |

## Developer and contributor

| Document | Description |
|---|---|
| [Architecture](architecture.md) | System diagram, package layout, request lifecycle. |
| [Modules](modules.md) | One-line purpose for every Go package. |
| [Algorithms](algorithms.md) | BM25, retry, circuit breaker, fallback, cost, vault, audit chain, HMAC, workflow DAG execution, retention. |
| [Design Decisions](design-decisions.md) | The summary index into the ADRs. |
| [ADRs](adr/README.md) | Architecture Decision Records. |
| [Development](development.md) | Setup, layout, Make targets, OpenAPI generator, migrations. |
| [Testing](testing.md) | Test layers, helpers, race detection, coverage. |
| [API Reference — Generator](api-reference.md#generator) | How `api/openapi.yaml` is produced. |

## Quick links

- **OpenAPI spec**: [`api/openapi.yaml`](../api/openapi.yaml)
- **Server health**: `GET /health`
- **Server readiness**: `GET /ready`
- **Prometheus metrics**: `GET /metrics`
- **Audit chain verify**: `GET /api/v1/audit/verify`
- **CLI help**: `./promptsheon help`
- **Server help**: `./promptsheond` (the server has no `--help`; configuration is via env vars — see [Configuration](configuration.md))
- **Makefile targets**: `make help`

## Authoring guide

If you are adding a new doc:

- One sentence in the first paragraph stating what the doc is for.
- One H1 (`#`), then H2s (`##`). No H3-H6.
- Tables for parallel data, not bullet lists.
- Code blocks must have a language tag (` ```bash `, ` ```go `, ` ```json `, ` ```text `, etc.).
- Use the [Glossary](glossary.md) terms verbatim. No synonyms.
- Link with relative paths inside `docs/`. Use `../api/openapi.yaml` for the OpenAPI spec.
- End with a "See also" section if the doc is referenced from elsewhere.
- Add a row to this index, in the right audience group.

If you are changing a doc:

- Update the "Last reviewed" footer to the current date.
- Run `make fmt` and `make lint` if you have them. Docs do not have a linter step in CI yet; a `markdownlint` config is a future addition.
