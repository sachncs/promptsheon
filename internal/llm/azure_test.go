package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAzureComplete(t *testing.T) {
	// Mock Azure OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify api-key header
		if r.Header.Get("api-key") == "" {
			t.Error("expected api-key header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}

		// Verify URL pattern
		if r.URL.Path != "/openai/deployments/gpt-4/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Errorf("unexpected api-version: %s", r.URL.Query().Get("api-version"))
		}

		resp := azureResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Delta *struct {
					Content string `json:"content"`
				} `json:"delta,omitempty"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{Content: "Hello from Azure!"},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Pass full server URL as the resource
	azure := NewAzure(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL, // includes http:// scheme
		Extra: map[string]string{
			"deployment":  "gpt-4",
			"api_version": "2024-02-15-preview",
		},
	})

	resp, err := azure.Complete(context.Background(), &Request{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Azure!" {
		t.Errorf("expected 'Hello from Azure!', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("expected 8 total tokens, got %d", resp.Usage.TotalTokens)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", resp.Model)
	}
}

func TestAzureName(t *testing.T) {
	azure := NewAzure(ProviderConfig{})
	if azure.Name() != "azure" {
		t.Errorf("expected name 'azure', got %q", azure.Name())
	}
}
