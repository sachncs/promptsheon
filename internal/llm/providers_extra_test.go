package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAnthropicDefaultBaseURL(t *testing.T) {
	a := NewAnthropic(ProviderConfig{})
	if !strings.HasPrefix(a.baseURL, "https://") {
		t.Errorf("expected https base URL, got %q", a.baseURL)
	}
}

func TestAnthropicName(t *testing.T) {
	if NewAnthropic(ProviderConfig{}).Name() != "anthropic" {
		t.Error(`expected "anthropic"`)
	}
}

func TestAnthropicCompleteHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the Anthropic-specific headers and path.
		if r.Header.Get("x-api-key") == "" {
			t.Error("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_01",
			"type": "message",
			"role": "assistant",
			"model": "claude-opus-4-7",
			"content": [{"type": "text", "text": "Hello back"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 12, "output_tokens": 7}
		}`))
	}))
	defer srv.Close()

	p := NewAnthropic(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), &Request{
		Model:     "claude-opus-4-7",
		MaxTokens: 64,
		Messages:  []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "Hello back" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 7 || resp.Usage.TotalTokens != 19 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %q", resp.StopReason)
	}
}

func TestAnthropicSystemPromptExtracted(t *testing.T) {
	var sawSystem bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 8192)
		n, _ := r.Body.Read(body)
		sawSystem = strings.Contains(string(body[:n]), `"system":[{`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_02",
			"type": "message",
			"role": "assistant",
			"model": "claude-opus-4-7",
			"content": [{"type": "text", "text": "ok"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 1, "output_tokens": 1}
		}`))
	}))
	defer srv.Close()

	p := NewAnthropic(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	if _, err := p.Complete(context.Background(), &Request{
		Model:    "claude-opus-4-7",
		Messages: []Message{{Role: "system", Content: "you are terse"}, {Role: "user", Content: "hi"}},
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !sawSystem {
		t.Fatal("expected system prompt block in request body")
	}
}

func TestAnthropicError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`))
	}))
	defer srv.Close()

	p := NewAnthropic(ProviderConfig{APIKey: "sk-bad", BaseURL: srv.URL})
	if _, err := p.Complete(context.Background(), &Request{
		Model: "claude-opus-4-7", Messages: []Message{{Role: "user", Content: "x"}},
	}); err == nil {
		t.Fatal("expected error on 401")
	}
}
