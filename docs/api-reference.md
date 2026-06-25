# API Reference

Base URL: `http://localhost:8080`

> **Source of truth.** The full OpenAPI 3.0 specification is at [`api/openapi.yaml`](../api/openapi.yaml). It is generated from the server's route table and is the only authoritative description of the wire format. This page is the human-readable summary.

## Generator

`api/openapi.yaml` is produced by [`scripts/genopenapi/`](../scripts/genopenapi/main.go). The generator:

- Walks `internal/api/server.go` for `mux.HandleFunc` calls.
- Walks `internal/api/handlers_*.go` for handler function signatures.
- Uses the `go/parser` AST to extract request body schemas.
- Is deterministic and idempotent.

Re-generate after any route or handler change:

```bash
make openapi          # writes api/openapi.yaml
make openapi-check    # writes it and fails if the file is dirty
```

CI runs `make openapi-check` on every PR. If the spec is out of date, the PR fails.

## Authentication

When `PROMPTSHEON_AUTH=true` (the default), all endpoints (except `/health`, `/ready`, `/metrics`, and the OAuth callback paths) require a Bearer token:

```
Authorization: Bearer ps_<api_key>
```

API keys are created via `POST /api/v1/apikeys` and the full key is shown once at creation time. The `?api_key=` query parameter is disabled. See [Security](security.md#authentication-and-authorisation).

## System

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Liveness probe. Returns `{"status":"ok","version":"...","uptime":"..."}`. |
| `GET` | `/ready` | Readiness probe. Returns `200` when the database is reachable. |
| `GET` | `/metrics` | Prometheus-format metrics (unauthenticated, intended for in-cluster scrape). |

## Endpoint groups

The OpenAPI spec is the authoritative list. The high-level groups, with the number of routes per group, are:

| Group | Routes | Description |
|---|---|---|
| `prompts` | 14 | Prompt CRUD, version history, run, stream, preview, similarity. |
| `agents` | 17 | Agent CRUD, import/export, execute, fork, deploy, archive, version history. |
| `contexts` | 8 | Conversation context CRUD, append/clear messages, assemble for LLM. |
| `datasets` | 8 | Dataset CRUD, CSV import, NDJSON export. |
| `eval` | 5 | Run evaluations, list results, compare runs, generate reports. |
| `guardrails` | 8 | Rule CRUD, content checks, violation tracking. |
| `alerts` | 8 | Alert rule CRUD, active alerts, resolve. |
| `reviews` | 5 | Pending reviews, approve, reject, comment. |
| `audit` | 3 | List, export, verify the hash chain. |
| `providers` | 3 | List, get, test LLM providers. |
| `vault` | 3 | Save, list, delete provider keys. |
| `workflows` | 5 | Run, list, get, list steps, cancel. |
| `traces` | 3 | List spans, get by ID, get full tree. |
| `logs` | 2 | Search spans, stream via SSE. |
| `metrics` | 5 | Summary, top prompts, top agents, dashboard, JSON metrics. |
| `users` | 5 | User CRUD. |
| `webhooks` | 3 | List, create, delete endpoints. |
| `auth` | 4 | API key CRUD, OAuth callbacks. |
| `abtesting` | ~6 | A/B test CRUD, run, results. |
| `optimizer` | ~3 | Prompt analysis, improvement suggestions. |
| `playground` | ~3 | Interactive prompt testing. |
| `collab` | ~3 | Real-time collaborative editing sessions. |

> Counts are derived from `internal/api/server.go` and `internal/api/handlers_*.go`. They drift as the codebase evolves. For the canonical list, see [`api/openapi.yaml`](../api/openapi.yaml).

## Pagination

List endpoints accept `limit` (1–1000, default 50) and `offset` (default 0) query parameters. Responses are plain JSON arrays (not envelopes).

## Error format

All errors return:

```json
{"error": "description of the error"}
```

Common status codes:

| Code | Meaning |
|---|---|
| `400` | Bad request — the body or query string is invalid. |
| `401` | Unauthorized — missing or invalid API key. |
| `403` | Forbidden — the key is valid but does not have the required role. |
| `404` | Not found. |
| `409` | Conflict — typically a version mismatch on update. |
| `413` | Payload too large — the request body exceeds 10 MB. |
| `429` | Too many requests — the rate limiter rejected the call. |
| `500` | Internal error. |
| `503` | Service unavailable — the database is unreachable. |

## Idempotency

`POST` endpoints that create resources are not idempotent today. A future ADR will introduce an `Idempotency-Key` header for clients that need safe retry.

## Versioning

The API is at `v1` and is stable within the `0.x` series. Breaking changes will be introduced in a `v2` mount, not in-place. The current schema version for the prompt binding is `0.3.0` (see ADR [0009](adr/0009-prompt-binding-version-0-3-0.md)).

## See also

- [OpenAPI spec](../api/openapi.yaml) — the source of truth
- [SDK](sdk.md) — Go client library
- [Security](security.md) — auth and error semantics
- [Algorithms](algorithms.md) — implementation details
