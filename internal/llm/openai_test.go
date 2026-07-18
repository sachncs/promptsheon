package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOpenAIDefaultBaseURL(t *testing.T) {
	o := NewOpenAI(ProviderConfig{})
	if !strings.HasPrefix(o.baseURL, "https://") {
		t.Errorf("expected https base URL, got %q", o.baseURL)
	}
}

func TestOpenAIName(t *testing.T) {
	if NewOpenAI(ProviderConfig{}).Name() != "openai" {
		t.Error(`expected "openai"`)
	}
}

func TestOpenAICompleteHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		// v3 Responses API path is /v1/responses.
		if !strings.HasSuffix(r.URL.Path, "/responses") {
			t.Errorf("expected /v1/responses, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "resp_01",
			"object": "response",
			"status": "completed",
			"model": "gpt-5",
			"output": [
				{
					"type": "message",
					"role": "assistant",
					"content": [{"type": "output_text", "text": "Hello back"}]
				}
			],
			"usage": {"input_tokens": 8, "output_tokens": 4}
		}`))
	}))
	defer srv.Close()

	p := NewOpenAI(ProviderConfig{APIKey: "sk-test", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), &Request{
		Model:     "gpt-5",
		MaxTokens: 64,
		Messages:  []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "Hello back" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 8 || resp.Usage.CompletionTokens != 4 || resp.Usage.TotalTokens != 12 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if resp.StopReason != "completed" {
		t.Fatalf("stop_reason = %q", resp.StopReason)
	}
}

func TestOpenAIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad api key","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()

	p := NewOpenAI(ProviderConfig{APIKey: "sk-bad", BaseURL: srv.URL})
	if _, err := p.Complete(context.Background(), &Request{
		Model: "gpt-5", Messages: []Message{{Role: "user", Content: "x"}},
	}); err == nil {
		t.Fatal("expected error on 401")
	}
}
