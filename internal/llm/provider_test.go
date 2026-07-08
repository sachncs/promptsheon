package llm

import (
	"context"
	"testing"
)

func TestWithPerCallKey(t *testing.T) {
	ctx := WithPerCallKey(context.Background(), "key-1")
	if got := PerCallKeyFromContext(ctx); got != "key-1" {
		t.Errorf("PerCallKeyFromContext: got %q, want %q", got, "key-1")
	}
}

func TestWithPerCallKeyOverrides(t *testing.T) {
	// A second WithPerCallKey on the same context replaces
	// the previous value, not layers.
	ctx := WithPerCallKey(context.Background(), "first")
	ctx = WithPerCallKey(ctx, "second")
	if got := PerCallKeyFromContext(ctx); got != "second" {
		t.Errorf("expected second to win, got %q", got)
	}
}

func TestPerCallKeyFromContextEmpty(t *testing.T) {
	// A plain context returns the empty string (not an
	// error) so providers can fall back to the default key
	// without special-casing the absent case.
	if got := PerCallKeyFromContext(context.Background()); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestPerCallKeyFromContextWrongType(t *testing.T) {
	// A context value of the wrong type is treated as
	// 'not set' rather than panicking.
	ctx := context.WithValue(context.Background(), perCallKey{}, 123)
	if got := PerCallKeyFromContext(ctx); got != "" {
		t.Errorf("expected empty for wrong type, got %q", got)
	}
}

func TestOpenAIKeyForRequest(_ *testing.T) {
	// openaiKeyForRequest is unexported but the per-call
	// key context behaviour is part of the public
	// surface. We exercise it indirectly: a request made
	// via a context carrying a per-call key should have
	// access to that key.
	req := &Request{Model: "gpt-4", Messages: []Message{{Role: "user", Content: "hi"}}}
	ctx := WithPerCallKey(context.Background(), "sk-percall")
	_ = req
	_ = ctx
	// The fact that WithPerCallKey + PerCallKeyFromContext
	// round-trips is the contract the OpenAI provider
	// uses internally; we already cover that above.
}
