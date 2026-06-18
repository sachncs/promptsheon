# API Reference

Base URL: `http://localhost:8080`

The full OpenAPI 3.0 specification is at [`api/openapi.yaml`](../api/openapi.yaml).

## Authentication

When `PROMPTSHEON_AUTH=true`, all endpoints (except `/health`, `/ready`, `/api/v1/apikeys`, and OAuth) require a Bearer token:

```
Authorization: Bearer ps_<api_key>
```

API keys are created via `POST /api/v1/apikeys` and the full key is shown once at creation time.

## System

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Health check — returns `{"status":"ok","version":"...","uptime":"..."}` |
| `GET` | `/ready` | Readiness check — returns 200 when database is accessible |

## Prompts

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/prompts` | List prompts (query: `search`, `status`, `tags`) |
| `POST` | `/api/v1/prompts` | Create a prompt |
| `GET` | `/api/v1/prompts/{id}` | Get a prompt by ID |
| `PUT` | `/api/v1/prompts/{id}` | Update a prompt |
| `DELETE` | `/api/v1/prompts/{id}` | Delete a prompt |
| `POST` | `/api/v1/prompts/{id}/deploy` | Deploy an approved prompt |
| `POST` | `/api/v1/prompts/{id}/archive` | Archive a prompt |
| `POST` | `/api/v1/prompts/{id}/run` | Execute prompt via LLM |
| `POST` | `/api/v1/prompts/{id}/stream` | Stream execution via SSE |
| `POST` | `/api/v1/prompts/{id}/preview` | Preview rendered prompt without LLM call |
| `GET` | `/api/v1/prompts/{id}/versions` | List prompt versions |
| `POST` | `/api/v1/prompts/{id}/restore` | Restore a previous version |
| `GET` | `/api/v1/prompts/similar` | Find similar prompts |

### Create Prompt

```json
POST /api/v1/prompts
{
  "name": "greeting",
  "description": "A greeting prompt",
  "content": "Hello {{name}}, welcome to {{product}}!",
  "variables": [
    {"name": "name", "type": "string", "required": true},
    {"name": "product", "type": "string", "required": true, "default": "Promptsheon"}
  ],
  "tags": ["onboarding"],
  "model_hint": "gpt-4",
  "metadata": {"team": "platform"}
}
```

### Run Prompt

```json
POST /api/v1/prompts/{id}/run
{
  "variables": {"name": "World"},
  "provider": "openai",
  "model": "gpt-4"
}
```

Response:

```json
{
  "content": "Hello World, welcome to Promptsheon!",
  "model": "gpt-4",
  "usage": {"prompt_tokens": 20, "completion_tokens": 8, "total_tokens": 28},
  "latency_ms": 1200
}
```

## Agents

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/agents` | List agents |
| `POST` | `/api/v1/agents` | Create an agent |
| `GET` | `/api/v1/agents/{id}` | Get an agent |
| `PUT` | `/api/v1/agents/{id}` | Update an agent |
| `DELETE` | `/api/v1/agents/{id}` | Delete an agent |
| `GET` | `/api/v1/agents/{id}/export` | Export agent (JSON or YAML) |
| `POST` | `/api/v1/agents/import-yaml` | Import agent from YAML |
| `POST` | `/api/v1/agents/{id}/fork` | Fork an agent |
| `POST` | `/api/v1/agents/{id}/execute` | Execute full agent workflow |
| `POST` | `/api/v1/agents/{id}/rerun` | Re-run an agent |
| `POST` | `/api/v1/agents/{id}/deploy` | Deploy agent |
| `POST` | `/api/v1/agents/{id}/archive` | Archive agent |
| `GET` | `/api/v1/agents/{id}/versions` | List agent versions |
| `POST` | `/api/v1/agents/{id}/restore` | Restore a previous version |
| `GET` | `/api/v1/agents/templates` | List agent templates |
| `POST` | `/api/v1/agents/validate` | Validate agent workflow DAG |

## Contexts

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/contexts` | Create a context |
| `GET` | `/api/v1/contexts` | List contexts |
| `GET` | `/api/v1/contexts/{id}` | Get a context |
| `PUT` | `/api/v1/contexts/{id}` | Update a context |
| `DELETE` | `/api/v1/contexts/{id}` | Delete a context |
| `POST` | `/api/v1/contexts/{id}/messages` | Append a message |
| `DELETE` | `/api/v1/contexts/{id}/messages` | Clear messages |
| `POST` | `/api/v1/contexts/{id}/assemble` | Assemble context for LLM |

## Datasets

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/datasets` | List datasets |
| `POST` | `/api/v1/datasets` | Create a dataset |
| `GET` | `/api/v1/datasets/{id}` | Get a dataset |
| `PUT` | `/api/v1/datasets/{id}` | Update a dataset |
| `DELETE` | `/api/v1/datasets/{id}` | Delete a dataset |
| `POST` | `/api/v1/datasets/import` | Import dataset |
| `POST` | `/api/v1/datasets/{id}/import-csv` | Import CSV test cases |
| `GET` | `/api/v1/datasets/{id}/export` | Export as NDJSON |

## Evaluations

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/eval/run` | Run evaluation against a dataset |
| `GET` | `/api/v1/eval/results` | List eval results (query: `prompt_hash`, `dataset_id`) |
| `GET` | `/api/v1/eval/report` | Get eval report (query: `prompt_hash`) |
| `GET` | `/api/v1/eval/compare` | Compare two eval runs |
| `GET` | `/api/v1/eval/runs` | List eval runs |

## Guardrails

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/guardrails/rules` | List guardrail rules |
| `POST` | `/api/v1/guardrails/rules` | Create a rule |
| `GET` | `/api/v1/guardrails/rules/{id}` | Get a rule |
| `PUT` | `/api/v1/guardrails/rules/{id}` | Update a rule |
| `DELETE` | `/api/v1/guardrails/rules/{id}` | Delete a rule |
| `POST` | `/api/v1/guardrails/check` | Check content against guardrails |
| `GET` | `/api/v1/guardrails/violations` | List violations |
| `PUT` | `/api/v1/guardrails/violations/{id}/resolve` | Resolve a violation |

## Alerting

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/alerts/rules` | List alert rules |
| `POST` | `/api/v1/alerts/rules` | Create alert rule |
| `GET` | `/api/v1/alerts/rules/{id}` | Get alert rule |
| `PUT` | `/api/v1/alerts/rules/{id}` | Update alert rule |
| `DELETE` | `/api/v1/alerts/rules/{id}` | Delete alert rule |
| `POST` | `/api/v1/alerts/notifications` | Add notification group |
| `GET` | `/api/v1/alerts/active` | List active alerts |
| `PUT` | `/api/v1/alerts/active/{id}/resolve` | Resolve an alert |

## Reviews

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/reviews` | List pending reviews |
| `POST` | `/api/v1/reviews` | Create a review |
| `PUT` | `/api/v1/reviews/{id}/approve` | Approve a review |
| `PUT` | `/api/v1/reviews/{id}/reject` | Reject a review |
| `POST` | `/api/v1/reviews/{id}/comment` | Add comment |

## Audit

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/audit` | List audit entries (query: `user_id`, `resource`, `action`, `since`, `until`, `limit`) |
| `GET` | `/api/v1/audit/export` | Export audit entries (query: `format`: `json`/`csv`) |
| `GET` | `/api/v1/audit/verify` | Verify audit chain integrity |

## Providers

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/providers` | List registered LLM providers |
| `GET` | `/api/v1/providers/{name}` | Get provider info |
| `POST` | `/api/v1/providers/{name}/test` | Test provider connectivity |

## Vault

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/vault/keys` | Save encrypted provider key |
| `GET` | `/api/v1/vault/keys` | List stored keys (metadata only) |
| `DELETE` | `/api/v1/vault/keys/{id}` | Delete a provider key |

## Workflows

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/workflows/run` | Run a workflow |
| `GET` | `/api/v1/workflows` | List workflows |
| `GET` | `/api/v1/workflows/{id}` | Get a workflow |
| `GET` | `/api/v1/workflows/{id}/steps` | Get workflow steps |
| `PUT` | `/api/v1/workflows/{id}/cancel` | Cancel a running workflow |

## Tracing & Logs

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/traces` | List trace spans |
| `GET` | `/api/v1/traces/{id}` | Get a span by ID |
| `GET` | `/api/v1/traces/tree/{trace_id}` | Get full trace tree |
| `GET` | `/api/v1/logs/search` | Search trace spans (query: `operation`, `service`, `trace_id`, `since`, `until`) |
| `GET` | `/api/v1/logs/stream` | Stream logs via SSE |

## Metrics

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/metrics/summary` | Metrics summary |
| `GET` | `/api/v1/metrics/top-prompts` | Top used prompts |
| `GET` | `/api/v1/metrics/top-agents` | Top used agents |
| `GET` | `/api/v1/metrics/dashboard` | Dashboard summary |
| `GET` | `/api/v1/metrics` | Prometheus-format metrics |
| `GET` | `/metrics` | Prometheus metrics endpoint (unauthenticated) |

## Users

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/users` | List users |
| `POST` | `/api/v1/users` | Create user |
| `GET` | `/api/v1/users/{id}` | Get user |
| `PUT` | `/api/v1/users/{id}` | Update user |
| `DELETE` | `/api/v1/users/{id}` | Delete user |

## Webhooks

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/webhooks` | List webhooks |
| `POST` | `/api/v1/webhooks` | Create webhook |
| `DELETE` | `/api/v1/webhooks/{id}` | Delete webhook |

## Error Format

All errors return:

```json
{"error": "description of the error"}
```

Common status codes: `400` (bad request), `401` (unauthorized), `403` (forbidden), `404` (not found), `409` (conflict), `500` (internal error).
