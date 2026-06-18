package llm

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestFallbackPrimarySuccess(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "primary response"}, nil
		},
	}
	fallback := &mockProvider{
		name: "fallback",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "fallback response"}, nil
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fb := NewFallback(primary, []Provider{fallback}, logger)

	resp, err := fb.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "primary response" {
		t.Errorf("content = %q, want %q", resp.Content, "primary response")
	}
}

func TestFallbackPrimaryFailsFallbackSucceeds(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return nil, errors.New("primary failed")
		},
	}
	fallback := &mockProvider{
		name: "fallback",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "fallback response"}, nil
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fb := NewFallback(primary, []Provider{fallback}, logger)

	resp, err := fb.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("content = %q, want %q", resp.Content, "fallback response")
	}
}

func TestFallbackAllFail(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return nil, errors.New("primary failed")
		},
	}
	fallback1 := &mockProvider{
		name: "fallback1",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return nil, errors.New("fallback1 failed")
		},
	}
	fallback2 := &mockProvider{
		name: "fallback2",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return nil, errors.New("fallback2 failed")
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fb := NewFallback(primary, []Provider{fallback1, fallback2}, logger)

	_, err := fb.Complete(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	if !errors.Is(err, err) {
		t.Errorf("expected error to contain 'all providers failed'")
	}
}

func TestFallbackSkipsDuplicateProviders(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return nil, errors.New("primary failed")
		},
	}
	// fallback has same name as primary
	fallback := &mockProvider{
		name: "primary",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "should not reach"}, nil
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fb := NewFallback(primary, []Provider{fallback}, logger)

	_, err := fb.Complete(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error when primary and fallback have same name")
	}
}

func TestFallbackName(t *testing.T) {
	primary := &mockProvider{name: "openai"}
	fallback1 := &mockProvider{name: "anthropic"}
	fallback2 := &mockProvider{name: "ollama"}

	fb := NewFallback(primary, []Provider{fallback1, fallback2}, nil)

	expected := "fallback(openai,anthropic,ollama)"
	if fb.Name() != expected {
		t.Errorf("Name() = %q, want %q", fb.Name(), expected)
	}
}

func TestParseFallbackProviders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single",
			input:    "anthropic",
			expected: []string{"anthropic"},
		},
		{
			name:     "multiple",
			input:    "anthropic,ollama",
			expected: []string{"anthropic", "ollama"},
		},
		{
			name:     "with spaces",
			input:    "anthropic , ollama , azure",
			expected: []string{"anthropic", "ollama", "azure"},
		},
		{
			name:     "with empty parts",
			input:    "anthropic,,ollama,",
			expected: []string{"anthropic", "ollama"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFallbackProviders(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("len = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
