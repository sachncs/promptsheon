// Package llm provides a provider-agnostic abstraction for calling large
// language models. Implementations live in this package; consumers depend
// only on the Provider interface.
package llm

import (
	"context"
	"time"

	"promptsheon/internal/models"
)

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Complete sends a prompt to the model and returns the response.
	Complete(ctx context.Context, req *Request) (*Response, error)

	// Name returns the provider identifier (e.g. "openai", "anthropic").
	Name() string
}

// Request holds the inputs for a single LLM call.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Stream      bool      `json:"-"`
}

// Message is a single message in a conversation.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// Response holds the output from an LLM call.
type Response struct {
	Content          string        `json:"content"`
	Usage            models.Usage  `json:"usage"`
	Model            string        `json:"model"`
	StopReason       string        `json:"stop_reason,omitempty"`
	Latency          time.Duration `json:"-"`
	TimeToFirstToken time.Duration `json:"-"` // time from request to first token (for streaming)
}

// ProviderConfig holds configuration for constructing a provider.
type ProviderConfig struct {
	APIKey  string            `json:"api_key"`
	BaseURL string            `json:"base_url,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"` // provider-specific config (e.g. deployment, api_version)
}
