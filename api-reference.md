---
layout: page
title: API reference
subtitle: Every public HTTP endpoint, with request and response shapes.
---

# API reference

Promptsheon exposes a JSON-over-HTTP API on the same port as the UI (`8080`). The frontend's Next.js config rewrites every `/api/*` request from `localhost:3000` to `localhost:8080`, so browser code can use relative paths.

The full OpenAPI 3.1 spec is auto-emitted at `GET /api/openapi.json` once the server boots. This page is the human-readable summary.

## Conventions

- **Content type.** All requests and responses use `application/json`.
- **Error shape.** Every error returns `{"error": {"code": "<MACHINE_CODE>", "message": "<human-readable>"}}`. See [Error codes](#error-codes).
- **Authentication.** When `PROMPTSHEON_AUTH=true`, every non-public request needs an `Authorization: Bearer <token>` header. The token is sha256-hashed and looked up in the `api_keys` table. See [Authentication](#authentication).
- **Request id.** Every response carries an `X-Request-Id` header. The header value is echoed if the request supplied one.
- **Pagination.** List endpoints accept `?page=N&pageSize=M`. The default page size is `25`. The response is always `{items: [...], total: N}`.

## Authentication

```http
POST /api/executions HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Content-Type: application/json
```

When `PROMPTSHEON_AUTH=false` (the development default), the request can include `X-User-Id` and `X-Org-Id` headers instead. In production, set `PROMPTSHEON_AUTH=true` and supply a Bearer token.

The server also accepts `Authorization: SVID <token>` for agent identities. See [Security]({{ '/security/' | relative_url }}).

## Health

### `GET /api/health`

Returns the server's liveness state. Always `200 OK`.

```json
{
  "status": "ok",
  "service": "promptsheon-server",
  "version": "0.4.2",
  "uptimeSec": 1234
}
```

### `GET /api/audit/state` and `GET /api/audit/verify`

Both endpoints are publicly callable (no auth required) so the documentation links work in a fresh browser. `/api/audit/verify` returns:

```json
{
  "valid": true,
  "frameCount": 1024,
  "lastHash": "abc123…"
}
```

If any frame is tampered with, `valid` becomes `false` and `brokenAt` reports the index of the first bad frame.

## Onboarding

### `POST /api/bootstrap/admin`

Creates the first admin user and organisation. Called once during onboarding.

```http
POST /api/bootstrap/admin
Content-Type: application/json

{
  "userName": "Alice",
  "userEmail": "alice@example.com",
  "orgName": "Acme"
}
```

The response carries the session cookie that the rest of the API expects.

## Workspaces

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/workspaces` | List workspaces in the active organisation. |
| `POST` | `/api/workspaces` | Create a workspace. |
| `GET` | `/api/workspaces/:id` | Fetch one workspace. |
| `PUT` | `/api/workspaces/:id` | Update a workspace. |
| `DELETE` | `/api/workspaces/:id` | Delete a workspace. |

```http
POST /api/workspaces
Content-Type: application/json

{
  "name": "Refund triage",
  "description": "Customer-facing refund flows"
}
```

## Projects, capabilities, versions, releases

The same CRUD shape applies. The full list lives in [`packages/server/API.md`](https://github.com/sachncs/promptsheon/blob/master/packages/server/API.md).

## Executions

### `POST /api/executions`

Execute a capability release. Without an `Accept` header, returns buffered JSON. With `Accept: text/event-stream`, streams per-node events:

```
event: execution_start
data: {"executionId":"e_123","manifestHash":"…"}

event: node_start
data: {"executionId":"e_123","nodeId":"plan"}

event: node_complete
data: {"executionId":"e_123","nodeId":"plan","output":"…"}

event: execution_complete
data: {"executionId":"e_123","output":"…","tokens":1234}

event: done
data: [DONE]
```

### `GET /api/executions/:id`

Fetch an execution by id. Returns the manifest, the inputs, the per-node outputs, and the trace.

### `POST /api/executions/:id/replay`

Re-run an execution with the same manifest, model, environment, and inputs. The new execution is linked to the original via `replay_of`; the original's `replay_count` is incremented.

### `GET /api/executions/:id/replays`

List every replay attempt with outcome and diff.

## Eval

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/eval/run` | Run an eval suite against a capability. |
| `GET` | `/api/eval/runs` | List past eval runs. |
| `POST` | `/api/repos/:id/eval-gate` | External-CI callable gate. Returns `{ok, score, regressions, suites}`. |

## Repositories, commits, merge requests

The full repo surface (`/api/repos`, `/api/repos/:id/branches`, `/api/repos/:id/commits`, `/api/merge-requests`, …) is documented in [`packages/server/API.md`](https://github.com/sachncs/promptsheon/blob/master/packages/server/API.md). Repos are content-addressed and signed.

## Cost and budgets

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/admin/cost-forecast` | OLS linear-regression projection per organisation. |
| `POST` | `/api/admin/budgets` | Create a budget. |
| `GET` | `/api/admin/budgets` | List budgets for the active organisation. |

## Firewall sidecar

If you mount the firewall sidecar (`pnpm --filter @promptsheon/server firewall`), it exposes `POST /v1/chat/completions` on a separate port. It forwards to the upstream URL after running the scanner. Decisions are written to the audit chain so `/api/audit/verify` covers sidecar traffic.

## Error codes

| Code | HTTP status | Meaning |
|------|-------------|---------|
| `UNAUTHORIZED` | 401 | Missing or invalid Bearer / SVID header. |
| `INVALID_SVID` | 401 | SVID signature or freshness check failed. |
| `SVID_PUBLIC_KEY_NOT_CONFIGURED` | 503 | SVID auth requested but `PROMPTSHEON_SVID_PUBLIC_KEY_PEM` is not set. |
| `NOT_ORG_MEMBER` | 403 | The caller is not a member of the named organisation. |
| `APPROVAL_REQUIRED` | 409 | The release needs two non-creator approvals before activation. |
| `MANIFEST_NOT_FOUND` | 404 | The manifest hash is not in the CAS. |
| `VALIDATION_ERROR` | 422 | The request body failed Zod validation. |
| `INTERNAL_ERROR` | 500 | Catch-all for unexpected failures. The server logs the stack trace. |

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
| Copy a working snippet | [Recipes]({{ '/recipes/' | relative_url }}) |
