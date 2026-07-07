# Promptsheon Go SDK

`github.com/sachncs/promptsheon/sdk` is the official Go client
for the Promptsheon REST API. It targets the `0.x` line of the
server and is regenerated on every release to match the OpenAPI
spec at `api/openapi.yaml`.

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

func main() {
    client := sdk.New("http://localhost:8080", "ps_...")
    ctx := context.Background()

    prompt, err := client.CreatePrompt(ctx, &sdk.CreatePromptRequest{
        Name:    "greeting",
        Content: "Hello {{name}}, welcome to {{product}}!",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("created %s (id=%s)\n", prompt.Name, prompt.ID)

    resp, err := client.RunPrompt(ctx, prompt.ID, &sdk.RunPromptRequest{
        Variables: map[string]string{"name": "world", "product": "Promptsheon"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Content)
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
prompt, err := client.GetPrompt(ctx, "missing")
if err != nil {
    var apiErr *sdk.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Status {
        case 401:
            log.Println("bad key")
        case 404:
            log.Println("not found")
        default:
            log.Printf("server said: %s", apiErr.Message)
        }
    }
    return err
}
```

## API coverage

The SDK currently exposes the high-traffic read/write surface:

| Resource | Methods |
|---|---|
| Prompts | `ListPrompts`, `GetPrompt`, `CreatePrompt`, `UpdatePrompt`, `DeletePrompt`, `RunPrompt`, `DeployPrompt`, `ArchivePrompt` |
| Agents  | `ListAgents`, `GetAgent` |
| Health  | `Health` |
| Providers | `ListProviders` |

The full HTTP surface is documented in `api/openapi.yaml`; if
you need a method the SDK doesn't expose yet, file an issue or
use `http.Post` against the documented endpoint while a typed
wrapper is being added.

## Testing

The SDK ships with a table-driven test suite that exercises
each method against an `httptest` server:

```bash
go test -race -count=1 ./sdk/...
```

The tests do not require a running promptsheond.

## Versioning

The SDK follows the server's [SemVer](https://semver.org/) tag.
A breaking change to the wire format is a major version bump of
both the server and the SDK module.
