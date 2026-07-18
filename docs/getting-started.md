# Getting Started

## Prerequisites

- Go 1.26 or later
- SQLite (included — no separate install needed via `modernc.org/sqlite`)
- golangci-lint (optional, for linting)

## Build

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon

# Build both binaries
make build

# Or individually
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon ./cmd/promptsheon
```

## Run the Server

```bash
# Default configuration (localhost:8080, promptsheon.db)
./promptsheond

# Custom configuration
PROMPTSHEON_ADDR=:9090 \
PROMPTSHEON_DB_PATH=./data.db \
PROMPTSHEON_LOG_LEVEL=debug \
./promptsheond
```

The server starts and exposes a REST API on the configured address.

## Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok","version":"0.3.0","uptime":"2.3s"}
```

## Create a Prompt

```bash
curl -X POST http://localhost:8080/api/v1/prompts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "greeting",
    "description": "A simple greeting prompt",
    "content": "Hello {{name}}, welcome to {{product}}!",
    "variables": [
      {"name": "name", "type": "string", "required": true},
      {"name": "product", "type": "string", "required": true, "default": "Promptsheon"}
    ],
    "tags": ["onboarding", "demo"]
  }'
```

## Run a Prompt

```bash
# Replace {id} with the prompt ID from the create response
curl -X POST http://localhost:8080/api/v1/prompts/{id}/run \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {"name": "World"},
    "provider": "openai",
    "model": "gpt-4"
  }'
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

## Preview Without Execution

```bash
curl -X POST http://localhost:8080/api/v1/prompts/{id}/preview \
  -H "Content-Type: application/json" \
  -d '{"variables": {"name": "World"}}'
```

Returns the rendered prompt text and estimated token count without calling an LLM.

## Create an Agent

```bash
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "research-agent",
    "description": "Searches and summarizes",
    "steps": [
      {
        "id": "step1",
        "prompt_id": "{prompt_id}",
        "depends_on": [],
        "output_key": "search_result"
      },
      {
        "id": "step2",
        "prompt_id": "{prompt_id_2}",
        "depends_on": ["step1"],
        "output_key": "summary"
      }
    ]
  }'
```

## Use the CLI

```bash
# List prompts
./promptsheon prompts list

# Get a specific prompt
./promptsheon prompts get {id}

# Run a prompt
./promptsheon prompts run {id} --var name=World --provider openai --model gpt-4
```

## Next Steps

- [Configuration](configuration.md) — All environment variables and options
- [API Reference](api-reference.md) — Full endpoint documentation
- [LLM Providers](llm-providers.md) — Set up OpenAI, Anthropic, Azure, or Ollama
- [Workflows](workflows.md) — Build multi-step agent workflows
- [Harness engineering](harness.md) and [Evaluations (v0.1.0)](eval.md) — datasets, preconditions, eval runs
- [Guardrails](guardrails.md) — Enforce content policies
- [Security](security.md) — the threat model and the operator checklist
- [Architecture](architecture.md) — the system diagram and package layout
- [Modules](modules.md) — one-line purposes for every Go package
- [Algorithms](algorithms.md) — BM25, retry, circuit breaker, audit chain, vault, HMAC
- [Design Decisions](design-decisions.md) — and the [ADRs](adr/README.md)
- [CLI](cli.md) — the `promptsheon` client binary
- [SDK](sdk.md) — the Go client library
- [Observability](observability.md) — logs, traces, metrics, retention
- [Troubleshooting](troubleshooting.md) — common problems
- [Glossary](glossary.md) — terminology reference
- [FAQ](faq.md) — frequently asked questions
- [Development](development.md) — for contributors
- [Testing](testing.md) — test layers and conventions
