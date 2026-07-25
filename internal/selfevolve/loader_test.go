package selfevolve

import (
	"context"
	"testing"
)

func TestCasPromptLoader_LoadMissing(t *testing.T) {
	l := NewCasPromptLoader()
	if _, err := l.LoadPrompt(context.Background(), "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatalf("expected error on missing prompt")
	}
}

func TestCasPromptLoader_WriteThenLoad(t *testing.T) {
	l := NewCasPromptLoader()
	hash, err := l.WritePrompt(context.Background(), "hello prompt")
	if err != nil {
		t.Fatalf("WritePrompt: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	got, err := l.LoadPrompt(context.Background(), hash)
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	if string(got) != "hello prompt" {
		t.Errorf("got %q, want hello prompt", string(got))
	}
}

func TestCasPromptLoader_DedupByContent(t *testing.T) {
	l := NewCasPromptLoader()
	h1, err := l.WritePrompt(context.Background(), "same content")
	if err != nil {
		t.Fatalf("WritePrompt 1: %v", err)
	}
	h2, err := l.WritePrompt(context.Background(), "same content")
	if err != nil {
		t.Fatalf("WritePrompt 2: %v", err)
	}
	if h1 != h2 {
		t.Errorf("identical content produced different hashes: %s vs %s", h1, h2)
	}
}

func TestCasPromptLoader_RespectsContextCancel(t *testing.T) {
	l := NewCasPromptLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.LoadPrompt(ctx, "x"); err == nil {
		t.Errorf("LoadPrompt on cancelled ctx: expected error")
	}
	if _, err := l.WritePrompt(ctx, "x"); err == nil {
		t.Errorf("WritePrompt on cancelled ctx: expected error")
	}
}
