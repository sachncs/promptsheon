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

func TestNewAnthropicDefaultBaseURL(t *testing.T) {
	a := NewAnthropic(ProviderConfig{})
	if !strings.HasPrefix(a.baseURL, "https://") {
		t.Errorf("expected https base URL, got %q", a.baseURL)
	}
	if a.client.Timeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", a.client.Timeout)
	}
}

func TestAnthropicName(t *testing.T) {
	if NewAnthropic(ProviderConfig{}).Name() != "anthropic" {
		t.Error("expected 'anthropic'")
	}
}

func TestAnthropicCompleteHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the Anthropic-specific headers.
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("expected x-api-key header")
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %q", r.URL.Path)
		}
		// Confirm system message was extracted to system
		// field, not in messages.
		var body struct {
			Model    string `json:"model"`
			System   string `json:"system"`
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.System != "you are helpful" {
			t.Errorf("expected system prompt, got %q", body.System)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Errorf("expected only user message, got %+v", body.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]string{{"text": "hello"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 3, "output_tokens": 2},
		})
	}))
	defer srv.Close()

	a := NewAnthropic(ProviderConfig{APIKey: "sk-ant-test", BaseURL: srv.URL})
	resp, err := a.Complete(context.Background(), &Request{
		Model:     "claude-3",
		MaxTokens: 100,
		Messages: []Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("Content: got %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 3 {
		t.Errorf("PromptTokens: got %d", resp.Usage.PromptTokens)
	}
}

func TestAnthropicCompleteServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	a := NewAnthropic(ProviderConfig{APIKey: "sk-ant-test", BaseURL: srv.URL})
	_, err := a.Complete(context.Background(), &Request{
		Model:    "claude-3",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got %v", err)
	}
}

func TestOllamaNew(t *testing.T) {
	o := NewOllama(ProviderConfig{})
	if o.baseURL == "" {
		t.Error("expected non-empty base URL")
	}
}

func TestOllamaName(t *testing.T) {
	if NewOllama(ProviderConfig{}).Name() != "ollama" {
		t.Error("expected 'ollama'")
	}
}

func TestOllamaComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"hi"},"done":true,"prompt_eval_count":3,"eval_count":2}`))
	}))
	defer srv.Close()

	o := NewOllama(ProviderConfig{BaseURL: srv.URL})
	resp, err := o.Complete(context.Background(), &Request{
		Model:    "llama2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "hi" {
		t.Errorf("Content: got %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 3 {
		t.Errorf("PromptTokens: got %d", resp.Usage.PromptTokens)
	}
}

func TestOllamaCompleteServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()

	o := NewOllama(ProviderConfig{BaseURL: srv.URL})
	_, err := o.Complete(context.Background(), &Request{
		Model:    "llama2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestNvidiaNew(t *testing.T) {
	n := NewNvidia(ProviderConfig{})
	if n.baseURL == "" {
		t.Error("expected non-empty base URL")
	}
}

func TestNvidiaName(t *testing.T) {
	if NewNvidia(ProviderConfig{}).Name() != "nvidia" {
		t.Error("expected 'nvidia'")
	}
}

func TestNvidiaComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	defer srv.Close()

	n := NewNvidia(ProviderConfig{APIKey: "nv-test", BaseURL: srv.URL})
	resp, err := n.Complete(context.Background(), &Request{
		Model:    "meta/llama",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "hi" {
		t.Errorf("Content: got %q", resp.Content)
	}
}
