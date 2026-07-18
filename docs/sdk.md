# SDKs

Promptsheon ships three SDKs that all target the same REST API
documented in [`api/openapi.yaml`](../../api/openapi.yaml):

| SDK | Path | Status |
|---|---|---|
| Go | `sdk/` | First-class; tracked with the server |
| Python | `sdk/python/` | Beta; mirrors the Go surface |
| TypeScript | `sdk/typescript/` | Beta; hand-written until codegen lands |

All three expose the same surface — workspace, project, capability,
version, release, approval, execution — and talk to the daemon over
`/api/v1/...`. The Go SDK is the source of truth; the Python and
TypeScript SDKs follow.

## Go SDK

```go
import "github.com/sachncs/promptsheon/sdk"

client := sdk.New("http://localhost:8080", "ps_...")
rel, _ := client.CreateRelease(ctx, "v1", sdk.CreateReleaseRequest{Environment: "prod"})
client.Vote(ctx, rel.ID, sdk.VoteRequest{Identity: "alice", Decision: "approve"})
client.Activate(ctx, rel.ID)
out, _ := client.Invoke(ctx, rel.ID, sdk.InvokeRequest{Model: "claude-opus-4"})
```

See `sdk/README.md` for the full surface and `examples/basic/main.go`
for an end-to-end demo.

## Python SDK

```python
from promptsheon import Client, ClientConfig

client = Client(ClientConfig(base_url="http://localhost:8080", api_key="ps_..."))
caps = client.list_capabilities("proj-1")
result = client.invoke_release("rel-1", inputs={"q": "hello"})
```

See `sdk/python/README.md`.

## TypeScript SDK

```typescript
import { PromptsheonClient } from "@promptsheon/typescript";

const client = new PromptsheonClient({ baseUrl: "http://localhost:8080", apiKey: "ps_..." });
await client.listCapabilities("proj-1");
await client.invokeRelease("rel-1", { inputs: { q: "hello" } });
```

See `sdk/typescript/README.md`.

## Common conventions

- All SDKs add `Authorization: Bearer <api-key>` when an api key is
  configured.
- 4xx and 5xx responses are surfaced as `*APIError` (Go),
  `PromptsheonAPIError` (Python), or thrown `Error` (TypeScript).
- Successful response bodies are decoded into the SDK's typed
  structs (Go) or `dict`/`unknown` (Python/TS until codegen lands).

## Regenerating from OpenAPI

The TypeScript SDK ships a placeholder `src/openapi.ts` until
`openapi-typescript` codegen lands (M3 follow-on). The Python SDK
is hand-mirrored from the Go surface. The Go SDK is the source of
truth — when the server gains a route, add the matching Go method
first and let the other SDKs follow in their own commits.
