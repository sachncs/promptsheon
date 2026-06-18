# Go SDK

The `sdk` package provides a Go client for the Promptsheon REST API.

## Installation

```bash
go get github.com/sachn-cs/promptsheon/sdk
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sachn-cs/promptsheon/sdk"
)

func main() {
    client := sdk.New("http://localhost:8080", "ps_your_api_key")
    ctx := context.Background()

    // List prompts
    prompts, err := client.ListPrompts(ctx)
    if err != nil {
        log.Fatal(err)
    }
    for _, p := range prompts {
        fmt.Printf("%s (v%d, %s)\n", p.Name, p.Version, p.Status)
    }
}
```

## Types

### Client

```go
client := sdk.New(baseURL, apiKey)
```

- `baseURL` — Server address (e.g., `http://localhost:8080`)
- `apiKey` — API key for authentication (empty string disables auth)

### Prompt

```go
type Prompt struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Content     string            `json:"content"`
    Variables   []Variable        `json:"variables"`
    Tags        []string          `json:"tags"`
    ModelHint   string            `json:"model_hint"`
    Binding     *ProviderBinding  `json:"binding,omitempty"`
    Version     int               `json:"version"`
    Status      string            `json:"status"`
    CreatedBy   string            `json:"created_by"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    Metadata    map[string]string `json:"metadata"`
}
```

### Variable

```go
type Variable struct {
    Name        string `json:"name"`
    Type        string `json:"type"`
    Required    bool   `json:"required"`
    Default     string `json:"default,omitempty"`
    Description string `json:"description"`
}
```

### Agent

```go
type Agent struct {
    ID          string      `json:"id"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Steps       []AgentStep `json:"steps"`
    Tools       []ToolRef   `json:"tools"`
    Status      string      `json:"status"`
}
```

### RunPromptRequest / RunPromptResponse

```go
req := &sdk.RunPromptRequest{
    Variables: map[string]string{"name": "World"},
    Provider:  "openai",
    Model:     "gpt-4",
}
resp, err := client.RunPrompt(ctx, promptID, req)
fmt.Println(resp.Content)
fmt.Printf("Tokens: %d, Latency: %dms\n", resp.Usage.TotalTokens, resp.LatencyMs)
```

## Methods

| Method | Description |
|---|---|
| `Health(ctx)` | Health check |
| `ListPrompts(ctx)` | List all prompts |
| `GetPrompt(ctx, id)` | Get a prompt by ID |
| `CreatePrompt(ctx, req)` | Create a prompt |
| `UpdatePrompt(ctx, id, req)` | Update a prompt |
| `DeletePrompt(ctx, id)` | Delete a prompt |
| `RunPrompt(ctx, id, req)` | Execute a prompt |
| `DeployPrompt(ctx, id)` | Deploy a prompt |
| `ArchivePrompt(ctx, id)` | Archive a prompt |
| `ListAgents(ctx)` | List all agents |
| `GetAgent(ctx, id)` | Get an agent by ID |
| `ListProviders(ctx)` | List LLM providers |

## Error Handling

All methods return errors. API errors are typed as `*sdk.APIError`:

```go
resp, err := client.GetPrompt(ctx, "nonexistent")
if err != nil {
    var apiErr *sdk.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.Status, apiErr.Message)
    }
}
```

## Context Support

All methods accept a `context.Context` for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

prompt, err := client.GetPrompt(ctx, id)
```
