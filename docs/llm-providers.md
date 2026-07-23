# LLM Providers

Promptsheon ships with two LLM providers in v0.1.x:
**OpenAI** and **Anthropic**. Both implement the
`internal/llm.Provider` interface and register a factory on
the `llm.Registry` at boot.

## Wiring a provider

Set the API key env var and the daemon picks the provider
up automatically:

```bash
# OpenAI.
export PROMPTSHEON_OPENAI_API_KEY="sk-..."

# Anthropic.
export PROMPTSHEON_ANTHROPIC_API_KEY="sk-ant-..."

# Optional base URL overrides (for proxies).
export PROMPTSHEON_OPENAI_BASE_URL="https://my-proxy.openai.local"
export PROMPTSHEON_ANTHROPIC_BASE_URL="https://my-proxy.anthropic.local"

./promptsheond
```

The daemon reads the env vars at boot; reload to pick up new
keys.

## Selecting a provider at runtime

A Release's Manifest resolves the provider name from the
`model_policy` artifact. The `invoke` path looks up the
resolved name on the `llm.Registry`; an unknown name returns
`502 Bad Gateway` with `{"provider_missing": true}` so
operators can tell the difference between "no provider
registered" and "provider call failed".

The request body for `/api/v1/versions/{id}/executions` and
`/api/v1/releases/{id}/invoke` carries the *inputs* only; the
provider + model come from the release's resolved plan.
Operators wanting to pick a different provider update the
Release's Manifest and create a new Version + Release.

## OpenAI

| Field | Value |
|-------|-------|
| SDK | `openai/openai-go/v3` (Responses API) |
| Env var | `PROMPTSHEON_OPENAI_API_KEY` |
| Base URL override | `PROMPTSHEON_OPENAI_BASE_URL` |
| Smoke test | `promptsheon provider test openai --model gpt-4o-mini` |

The Responses API is the supported surface; the older
Chat Completions API is not used.

## Anthropic

| Field | Value |
|-------|-------|
| SDK | `anthropics/anthropic-sdk-go` |
| Env var | `PROMPTSHEON_ANTHROPIC_API_KEY` |
| Base URL override | `PROMPTSHEON_ANTHROPIC_BASE_URL` |
| Smoke test | `promptsheon provider test anthropic --model claude-haiku-4-5` |

## Removed providers

Azure OpenAI, Ollama, and NVIDIA NIM were removed in v0.1.0.
To re-introduce any of them, implement the
`internal/llm.Provider` interface (one method,
`Complete(ctx, *Request) (*Response, error)`) and register a
factory on the `Registry` in `cmd/promptsheond/main.go`. The
registry is the integration boundary — no domain code needs
to change.

```go
r := llm.NewRegistry()
r.Register("myprovider", func(cfg llm.ProviderConfig) llm.Provider {
    return myProvider{cfg: cfg}
})
```

## Provider configuration

The `ProviderConfig` struct carries:

| Field | Type | Description |
|-------|------|-------------|
| `APIKey` | string | Provider API key. |
| `BaseURL` | string | Provider base URL. |
| `Extra` | map[string]string | Provider-specific config (e.g. Azure deployment, API version). |

The `llm.Registry` keeps a cache keyed by provider name; a
provider is constructed once per process. Tests that want to
reset the cache call `Registry.Configure(name, cfg)` which
invalidates the cached instance.

## Circuit breaker

`internal/llm/circuitbreaker.go` ships a circuit breaker
that wraps any `Provider` with success/failure tracking and
state transitions (`closed` / `open` / `half-open`). The
daemon doesn't wire it into the default providers in v0.1.x
because the OpenAI and Anthropic SDKs already retry on
transient errors; production tenants who need their own
policy wrap a `Provider` with `NewCircuitBreakerMiddleware` at
startup.

## Per-call API key override

A request that needs a per-call API key (different from the
registry-level one) calls `llm.WithPerCallKey(ctx, key)` on
the context. Providers that support per-call key injection
honour this; providers that don't fall back to the
registry-level key.

## See also

- [docs/configuration.md](configuration.md) — full env-var
  reference including TLS, leader-election, and OTel settings.
- [docs/security.md](security.md) — Vault + KMS-backed
  `KeyProvider` for the master key, plus the audit chain
  contract.
- [docs/getting-started.md](getting-started.md) — the 10-step
  walkthrough.