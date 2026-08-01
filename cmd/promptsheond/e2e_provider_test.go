//go:build e2e

// Verifies the //go:build e2e fence wires the in-memory LLM
// provider. Without -tags=e2e the corresponding e2e_provider.go
// is excluded and registerE2EProvider is the stub no-op.
package main

import (
	"github.com/sachncs/promptsheon/promptsheon/llm"
	"context"
	"log/slog"
	"os"
	"testing"

)

func TestE2EProvider_Fence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := llm.NewRegistry()
	raw := registerE2EProvider(reg, logger)
	p, ok := raw.(*llm.InMemoryProvider)
	if !ok || p == nil {
		t.Fatal("e2e provider must be installed under -tags=e2e")
	}
	if p.Calls() != 0 {
		t.Errorf("fresh provider must have 0 calls, got %d", p.Calls())
	}

	// Verify the provider is reachable through the registry.
	got, err := reg.Get("in-memory")
	if err != nil {
		t.Fatalf("registry lookup: %v", err)
	}
	if got.Name() != "in-memory" {
		t.Errorf("provider.Name = %q, want in-memory", got.Name())
	}

	// And a round-trip call increments the counter.
	_, err = got.Complete(context.TODO(), &llm.Request{
		Model:    "test-model",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if p.Calls() != 1 {
		t.Errorf("Calls = %d, want 1", p.Calls())
	}
}
