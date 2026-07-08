package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOpenAIKeyForRequestPrecedence pins the per-call key
// behaviour: a context with WithPerCallKey wins over the
// provider's default apiKey. Without the override, the
// default is used.
func TestOpenAIKeyForRequestPrecedence(t *testing.T) {
	o := NewOpenAI(ProviderConfig{APIKey: "sk-default"})
	if got := o.keyForRequest(context.Background()); got != "sk-default" {
		t.Errorf("default key: got %q", got)
	}
	ctx := WithPerCallKey(context.Background(), "sk-percall")
	if got := o.keyForRequest(ctx); got != "sk-percall" {
		t.Errorf("per-call key: got %q", got)
	}
}

// TestOpenAICompleteHappyPath drives the OpenAI provider
// against an httptest server that returns a canned chat-
// completions response. The provider's URL is pointed at
// the test server so the request never leaves the process.
func TestOpenAICompleteHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the auth header.
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("expected Bearer auth, got %q", got)
		}
		// Verify the path.
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]string{"role": "assistant", "content": "hello world"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{
				"prompt_tokens":     3,
				"completion_tokens": 2,
				"total_tokens":      5,
			},
		})
	}))
	defer srv.Close()

	o := NewOpenAI(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	resp, err := o.Complete(context.Background(), &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("Content: got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Errorf("Usage.TotalTokens: got %d", resp.Usage.TotalTokens)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("Model: got %q", resp.Model)
	}
}

func TestOpenAICompleteServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	o := NewOpenAI(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	_, err := o.Complete(context.Background(), &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected 429 in error, got %v", err)
	}
}

// TestOpenAIStreamHappyPath exercises the streaming path.
// The test server returns a single SSE chunk followed by
// [DONE] and the provider should return a Response with
// the chunk's content.
func TestOpenAIStreamHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	o := NewOpenAI(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	rc, err := o.Stream(context.Background(), &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf := make([]byte, 4096)
	n, _ := rc.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' in stream, got %q", got)
	}
}

func TestOpenAIStreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	o := NewOpenAI(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	_, err := o.Stream(context.Background(), &Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got %v", err)
	}
}

func TestStreamCloserIdempotent(t *testing.T) {
	// The streamCloser type's Close method must be
	// safe to call multiple times. We construct one
	// directly to exercise the contract.
	rc := &streamCloser{
		ReadCloser: &readCloserFunc{reader: strings.NewReader("hello")},
	}
	if err := rc.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Errorf("second close should be no-op, got %v", err)
	}
}

// readCloserFunc is a tiny test helper that wraps a strings.Reader
// as an io.ReadCloser. The Close method is a no-op so the
// streamCloser wrapper is the one being tested.
type readCloserFunc struct {
	reader interface {
		Read([]byte) (int, error)
	}
}

func (r *readCloserFunc) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *readCloserFunc) Close() error               { return nil }

func TestOpenAIName(t *testing.T) {
	if NewOpenAI(ProviderConfig{}).Name() != "openai" {
		t.Error("expected 'openai'")
	}
}

func TestNewOpenAIDefaultBaseURL(t *testing.T) {
	// When BaseURL is empty, the provider falls back to the
	// public OpenAI endpoint. We don't want the test to
	// actually hit that endpoint, so we just confirm the
	// value is non-empty and starts with https.
	o := NewOpenAI(ProviderConfig{})
	if !strings.HasPrefix(o.baseURL, "https://") {
		t.Errorf("expected https base URL, got %q", o.baseURL)
	}
}

func TestOpenAITimeout(t *testing.T) {
	// The client has a 120s timeout. A test that takes
	// longer than that would hang, so we just confirm the
	// client is configured.
	o := NewOpenAI(ProviderConfig{})
	if o.client.Timeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", o.client.Timeout)
	}
}
