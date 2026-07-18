# Promptsheon Go SDK

`github.com/sachncs/promptsheon/sdk` is the official Go client
for the Promptsheon REST API. It targets the `0.x` line of the
server and is regenerated on every release to match the OpenAPI
spec at `api/openapi.yaml`.

> **Forward-only.** v0.0.7 Prompt and Agent SDK methods are gone
> in v0.1.0. Use the Capability/Version/Release flow below.

## Install

```bash
go get github.com/sachncs/promptsheon/sdk
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sachncs/promptsheon/sdk"
)

const sampleHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func main() {
    client := sdk.New("http://localhost:8080", "ps_...")
    ctx := context.Background()

    // Server-side: workspace + project + capability + version are
    // assumed created (curl works fine; or use CreateWorkspace /
    // CreateCapability / AddVersion from this SDK). The flow below
    // drives the Release lifecycle end-to-end.

    rel, err := client.CreateRelease(ctx, "v1", sdk.CreateReleaseRequest{
        Environment: "prod",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("release=%s status=%s\n", rel.ID, rel.Status)

    // Vote as a non-creator identity (MakerChecker policy default).
    if _, err := client.Vote(ctx, rel.ID, sdk.VoteRequest{
        Identity: "alice",
        Decision: "approve",
    }); err != nil {
        log.Fatal(err)
    }

    // Activate, then invoke.
    if _, err := client.Activate(ctx, rel.ID); err != nil {
        log.Fatal(err)
    }
    out, err := client.Invoke(ctx, rel.ID, sdk.InvokeRequest{
        Inputs: map[string]any{"q": "hello"},
        Model:  "claude-opus-4",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("invoked: %s\n", out.ID)

    // Convenience: do all three in one call.
    // out, err := client.ApproveAndInvoke(ctx, rel.ID, "alice", sdk.InvokeRequest{
    //     Model: "claude-opus-4",
    // })
}
```

## Configuration

```go
// Default 30-second per-request timeout.
client := sdk.New("https://promptsheon.example.com", apiKey)

// Custom timeout or transport (for retry middleware, metrics
// instrumentation, mTLS, etc.).
client := sdk.NewWithHTTP("https://promptsheon.example.com", apiKey, &http.Client{
    Timeout: 10 * time.Second,
    Transport: myInstrumentedTransport,
})
```

`NewWithHTTP(nil, ...)` falls back to `http.DefaultClient`, which
has no timeout — set one explicitly in long-running processes.

## Error handling

`4xx` and `5xx` responses are returned as `*sdk.APIError`. The
SDK decodes the server's `{"error": "..."}` body into the
`Message` field, with two fallbacks for non-canonical shapes
(legacy `{"message": "..."}` and raw text):

```go
rel, err := client.GetRelease(ctx, "missing")
if err != nil {
    var apiErr *sdk.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Status {
        case 401:
            log.Println("bad key")
        case 404:
            log.Println("release not found")
        case 409:
            log.Println("quorum not satisfied")
        default:
            log.Printf("server said: %s", apiErr.Message)
        }
    }
    return err
}
```

## API coverage

The SDK exposes the high-traffic capability/release surface:

| Resource | Methods |
|---|---|
| Workspaces | `CreateWorkspace` |
| Capabilities | `CreateCapability` |
| Versions | `AddVersion` |
| Releases | `CreateRelease`, `GetRelease`, `ListReleases`, `Vote`, `Activate`, `Rollback`, `Invoke`, `Approval`, `ApproveAndInvoke` |
| Health | `Health` |
| Providers | `ListProviders` |

The full HTTP surface is documented in `api/openapi.yaml`; if
you need a method the SDK doesn't expose yet, file an issue or
use `http.Post` against the documented endpoint while a typed
wrapper is being added.

## Testing

```bash
go test -race -count=1 ./sdk/...
```

The tests do not require a running promptsheond.

## Versioning

The SDK follows the server's [SemVer](https://semver.org/) tag.
A breaking change to the wire format is a major version bump of
both the server and the SDK module.
