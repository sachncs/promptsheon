// Package llm provides a provider-agnostic abstraction for calling large
// language models. Implementations live in this package; consumers depend
// only on the Provider interface.
package llm

import (
	"context"
	"io"
	"time"
)

// Provider is the interface that all LLM backends must implement.
type Provider interface {
	// Complete sends a prompt to the model and returns the response.
	Complete(ctx context.Context, req *Request) (*Response, error)

	// Name returns the provider identifier (e.g. "openai", "anthropic").
	Name() string
}

// StreamingProvider extends Provider with streaming support.
type StreamingProvider interface {
	Provider

	// Stream sends a prompt to the model and returns a reader for streaming tokens.
	Stream(ctx context.Context, req *Request) (io.ReadCloser, error)
}

// TokenStream represents a single token in a streaming response.
type TokenStream struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
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
	Usage            Usage         `json:"usage"`
	Model            string        `json:"model"`
	StopReason       string        `json:"stop_reason,omitempty"`
	Latency          time.Duration `json:"-"`
	TimeToFirstToken time.Duration `json:"-"` // time from request to first token (for streaming)
}

// perCallKey is the context key under which a per-call API key is
// stashed. Providers that support per-call key injection (see
// PerCallKeyFromContext) prefer this over the registry-level key.
type perCallKey struct{}

// WithPerCallKey returns a new context with the per-call API key set.
// The key overrides the provider's default key for this single call.
func WithPerCallKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, perCallKey{}, key)
}

// PerCallKeyFromContext returns the per-call API key, or empty if none
// was set. Providers use this to honour prompt-level bindings.
func PerCallKeyFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(perCallKey{}).(string); ok {
		return v
	}
	return ""
}

// ProviderConfig holds configuration for constructing a provider.
type ProviderConfig struct {
	APIKey  string            `json:"api_key"`
	BaseURL string            `json:"base_url,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"` // provider-specific config (e.g. deployment, api_version)
}
