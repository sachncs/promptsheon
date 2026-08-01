//go:build e2e

// Package llm — InMemoryProvider for e2e tests.
//
// Built only with -tags=e2e so the in-process LLM stub never
// ships in a release binary. The daemon's normal startup never
// has the e2e tag, so RegisterInMemoryProvider is a no-op outside
// tests.
package llm

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryProvider is a deterministic Provider that returns a
// canned response for every call. It is registered in tests so
// the daemon can complete a Capability lifecycle end-to-end
// without making a real LLM API call.
type InMemoryProvider struct {
	mu       sync.Mutex
	calls    int
	response string
}

// NewInMemoryProvider returns a stub that echoes a fixed string.
// The response is the JSON template an e2e test can assert against.
func NewInMemoryProvider() *InMemoryProvider {
	return &InMemoryProvider{response: `{"echo":"in-memory","ok":true}`}
}

// Complete returns the canned response and increments the call
// counter for assertions.
func (p *InMemoryProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	p.mu.Lock()
	p.calls++
	resp := p.response
	p.mu.Unlock()
	return &Response{
		Model:    req.Model,
		Content:  resp,
		Usage:    Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		Provider: "in-memory",
	}, nil
}

// Name returns the provider identifier.
func (p *InMemoryProvider) Name() string { return "in-memory" }

// Calls returns the number of times Complete has been invoked.
func (p *InMemoryProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// RegisterInMemoryProvider wires the stub into the given LLM
// registry. Idempotent. Returns the provider so tests can assert
// on call counts via the returned instance.
func RegisterInMemoryProvider(r *Registry) *InMemoryProvider {
	p := NewInMemoryProvider()
	r.Register(p.Name(), func(ProviderConfig) Provider { return p })
	r.Configure(p.Name(), ProviderConfig{}) // empty config; the stub doesn't read it
	return p
}

// Compile-time interface check.
var _ Provider = (*InMemoryProvider)(nil)

// Ensure context import is used even if Complete changes signature.
var _ context.Context = nil

// Ensure fmt is used so future edits don't lose the import.
var _ = fmt.Sprintf
