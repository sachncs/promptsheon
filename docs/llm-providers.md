# LLM Providers

Promptsheon provides a unified abstraction over multiple LLM providers. Configure one or more providers via environment variables and reference them in prompts, agents, and evaluations.

## Provider List

| Provider | Package | Environment Variables |
|---|---|---|
| OpenAI | `internal/llm/openai.go` | `OPENAI_API_KEY`, `OPENAI_BASE_URL` |
| Anthropic | `internal/llm/anthropic.go` | `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL` |
| Azure OpenAI | `internal/llm/azure.go` | `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_VERSION`, `AZURE_OPENAI_DEPLOYMENT` |
| Ollama | `internal/llm/ollama.go` | `OLLAMA_BASE_URL` (default: `http://localhost:11434`) |
| NVIDIA NIM | `internal/llm/nvidia.go` | `NVIDIA_API_KEY`, `NVIDIA_BASE_URL` |

The list of providers is registered in `internal/llm/registry.go`. Adding a new provider is a single `r.Register("name", func(cfg) Provider { return NewMyProvider(cfg) })` line.

## OpenAI

```bash
export OPENAI_API_KEY="sk-..."
```

Supported models: `gpt-4`, `gpt-4-turbo`, `gpt-4o`, `gpt-3.5-turbo`, and all other OpenAI models.

### Example: Create prompt with OpenAI binding

```json
POST /api/v1/prompts
{
  "name": "summarizer",
  "content": "Summarize the following text: {{text}}",
  "binding": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key_ref": "openai-production"
  }
}
```

## Anthropic

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Supported models: `claude-3-opus`, `claude-3-sonnet`, `claude-3-haiku`, and newer Claude models.

## Azure OpenAI

```bash
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com/"
export AZURE_OPENAI_API_VERSION="2024-02-15-preview"
export AZURE_OPENAI_DEPLOYMENT="my-gpt4-deployment"   # the deployment name, not the model
```

Use the **deployment name** as the model parameter. Azure uses the same API format as OpenAI with a different base URL. The `api_key_ref` field in a prompt binding should reference a vault key tagged for Azure.

## Ollama (Local)

```bash
export OLLAMA_BASE_URL="http://localhost:11434"
```

Ollama runs locally with no API key required. Pull models first:

```bash
ollama pull llama3
ollama pull mistral
```

Then reference them as the model in requests. Ollama is ideal for development and testing without incurring API costs.

## NVIDIA NIM

```bash
export NVIDIA_API_KEY="..."
```

NVIDIA NIM provides optimized inference endpoints for popular models.

## Provider Vault

Store encrypted API keys in the vault so they are never exposed in environment variables or request bodies:

```bash
# Save a key
curl -X POST http://localhost:8080/api/v1/vault/keys \
  -H "Content-Type: application/json" \
  -d '{
    "provider_name": "openai",
    "key_name": "production",
    "key": "sk-..."
  }'

# List stored keys (metadata only, not the actual key)
curl http://localhost:8080/api/v1/vault/keys
```

The vault is encrypted with AES-256-GCM using `PROMPTSHEON_VAULT_KEY`. See [Algorithms — Vault](algorithms.md#vault-aes-256-gcm) and ADR [0004](adr/0004-aes-256-gcm-vault.md).

## Resilience

Every provider call goes through three middleware layers, in this order:

1. **Timeout** — `Timeouting` wraps the provider with a per-call context timeout.
2. **Circuit breaker** — when the provider fails repeatedly, calls are rejected with `ErrCircuitOpen` until a cooldown elapses. See [Algorithms — Circuit breaker](algorithms.md#circuit-breaker).
3. **Retry** — transient failures are retried with exponential backoff. See [Algorithms — Retry](algorithms.md#retry).

If the primary provider fails, the dispatcher walks a configured fallback chain. See [Algorithms — Fallback chain](algorithms.md#fallback-chain).

```bash
# Comma-separated fallback list
PROMPTSHEON_LLM_FALLBACK=anthropic,ollama
```

## Cost

The cost of a call is computed from a per-model pricing table in `internal/llm/cost.go` and recorded as a `llm_call_cost_usd_total` metric. See [Algorithms — Cost calculation](algorithms.md#cost-calculation) and [Observability](observability.md#metrics).

## Testing Providers

Test provider connectivity before use:

```bash
curl -X POST http://localhost:8080/api/v1/providers/openai/test \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o"}'
```

Response:

```json
{"provider": "openai", "status": "ok", "latency_ms": 850}
```

## Listing Providers

```bash
curl http://localhost:8080/api/v1/providers
```

Returns only providers that have valid credentials configured.
