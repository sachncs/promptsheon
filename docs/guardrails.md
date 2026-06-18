# Guardrails

Guardrails enforce content policies, safety rules, and operational limits on prompts and LLM outputs. They run both statically (before LLM calls) and at runtime (on responses).

## Rule Types

| Type | Description |
|---|---|
| `prompt_length` | Blocks prompts exceeding a character limit |
| `restricted_term` | Blocks content containing banned terms |
| `model_access` | Restricts which models can be used per environment |
| `hallucination_high` | Flags outputs with high hallucination scores |
| `format_invalid` | Validates output format (JSON, markdown, regex) |
| `cost_limit` | Blocks calls exceeding estimated cost |
| `latency_limit` | Flags responses exceeding latency threshold |
| `content_policy` | Enforces PII detection, harmful content blocking |

## Severity Levels

| Level | Description |
|---|---|
| `low` | Informational — logged but not blocking |
| `medium` | Warning — logged and may block in strict mode |
| `high` | Blocking — request is rejected |
| `critical` | Immediate block — logged as security event |

## Static Guardrails

Run before the LLM call. Check prompt content for violations:

### Prompt Length

```bash
curl -X POST http://localhost:8080/api/v1/guardrails/check \
  -H "Content-Type: application/json" \
  -d '{
    "content": "your prompt content here...",
    "model": "gpt-4o",
    "environment": "prod"
  }'
```

### Restricted Terms

Banned terms are checked case-insensitively against prompt content.

### Model Access Control

Restrict models by environment:

```json
{
  "prod": ["gpt-4", "claude-3-opus"],
  "dev": ["gpt-3.5-turbo", "gpt-4", "claude-3-haiku"]
}
```

Requests for disallowed models return a `model_access` violation.

## Runtime Guardrails

Run on LLM output after the call completes:

### Response Format Validation

Validates output matches expected format:

| Format | Check |
|---|---|
| `json` | Response starts with `{` or `[` |
| `markdown` | Response contains markdown structure characters |
| regex | Response matches the provided regex pattern |

### Content Policy

| Policy | Description |
|---|---|
| `no_pii` | Detects SSNs, credit card numbers, email addresses |
| `no_harmful` | Blocks content with harmful terms |

### Cost and Latency Limits

Block or flag calls exceeding configured thresholds:

```bash
# Cost limit: $0.05 per call
curl -X POST http://localhost:8080/api/v1/guardrails/check \
  -H "Content-Type: application/json" \
  -d '{
    "content": "...",
    "cost_limit": 0.05,
    "latency_limit": 5000
  }'
```

## Managing Rules

### Create a Rule

```bash
curl -X POST http://localhost:8080/api/v1/guardrails/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "block-profanity",
    "type": "restricted_term",
    "severity": "high",
    "enabled": true,
    "config": {
      "terms": ["badword1", "badword2"]
    }
  }'
```

### List Rules

```bash
curl http://localhost:8080/api/v1/guardrails/rules
```

### Update a Rule

```bash
curl -X PUT http://localhost:8080/api/v1/guardrails/rules/{id} \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'
```

### Delete a Rule

```bash
curl -X DELETE http://localhost:8080/api/v1/guardrails/rules/{id}
```

## Violations

### List Violations

```bash
curl http://localhost:8080/api/v1/guardrails/violations
```

### Resolve a Violation

```bash
curl -X PUT http://localhost:8080/api/v1/guardrails/violations/{id}/resolve
```

## Metrics

Guardrail activity is tracked via Prometheus metrics:

| Metric | Description |
|---|---|
| `guardrail_violations_total` | Total violations recorded |
| `guardrail_blocks_total` | Total blocked requests |
| `guardrail_passes_total` | Total passing checks |
