# LLM Providers

Promptsheon ships two first-class LLM providers in v0.1.x, each
implemented against the official SDK:

| Provider | SDK | Env vars |
|---|---|---|
| Anthropic | [`anthropics/anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go) | `PROMPTSHEON_ANTHROPIC_API_KEY`, `PROMPTSHEON_ANTHROPIC_BASE_URL` |
| OpenAI | [`openai/openai-go/v3`](https://github.com/openai/openai-go) (Responses API) | `PROMPTSHEON_OPENAI_API_KEY`, `PROMPTSHEON_OPENAI_BASE_URL` |

The provider registry in `internal/llm/registry.go` registers both by
default. Add a new provider by calling `r.Register("name", factory)`
on the registry — no daemon restart is needed if you swap factories
in code.

## Anthropic

```bash
export PROMPTSHEON_ANTHROPIC_API_KEY="sk-ant-..."
# Optional: override the base URL for a proxy or self-hosted gateway
# export PROMPTSHEON_ANTHROPIC_BASE_URL="https://api.anthropic.com"
```

Supported models: `claude-opus-4-*`, `claude-sonnet-4-*`,
`claude-3-5-sonnet-*`, `claude-3-5-haiku-*`, and any other model the
Anthropic Messages API accepts.

The provider translates `llm.Request.Messages` into the
Anthropic Messages format, extracting a single `system` prompt when
present and mapping user/assistant messages to the corresponding
blocks.

## OpenAI

```bash
export PROMPTSHEON_OPENAI_API_KEY="sk-..."
# Optional: override the base URL for a proxy or the Azure-compatible
# /openai/v1 Responses endpoint
# export PROMPTSHEON_OPENAI_BASE_URL="https://api.openai.com"
```

Supported models: any model that the OpenAI Responses API accepts
(`gpt-5`, `gpt-5.2`, `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`,
`gpt-4`, `gpt-3.5-turbo`, …).

The provider uses the v3 Responses API (not Chat Completions) and
joins `Request.Messages` into a single `Input` string; richer
message types can be added by introducing a per-message `Input`
list-builder if a call site needs them.

`Request.Stop` is intentionally not surfaced: the Responses API in
v3 only accepts a single stop sequence and the field is documented
as silently dropped. Callers that need deterministic truncation
should set `max_tokens` instead.

## Adding a new provider

Implement `llm.Provider`:

```go
type MyProvider struct{ ... }
func (p *MyProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) { ... }
func (p *MyProvider) Name() string { return "my-provider" }
```

Then register it:

```go
r := llm.NewRegistry()
r.Register("my-provider", func(cfg llm.ProviderConfig) llm.Provider {
    return &MyProvider{cfg: cfg}
})
r.Configure("my-provider", llm.ProviderConfig{APIKey: "..."})
```

The provider participates in the same flow as Anthropic and
OpenAI: the daemon's `WithInvoker` path consumes `llm.Provider`,
the CLI's `provider test` exercises it directly.

## Removed providers (v0.0.x)

Azure OpenAI, Ollama, and NVIDIA NIM were removed in v0.1.0
(forward-only cleanup). Operators who need an Ollama-compatible
local endpoint should run a proxy that exposes the OpenAI Responses
API and set `PROMPTSHEON_OPENAI_BASE_URL` accordingly.
