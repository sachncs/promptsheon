package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"promptsheon/internal/models"
)

// Ollama implements Provider for the Ollama local API.
type Ollama struct {
	baseURL string
	client  *http.Client
}

// NewOllama creates an Ollama provider.
func NewOllama(cfg ProviderConfig) *Ollama {
	base := cfg.BaseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	return &Ollama{
		baseURL: base,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

func (o *Ollama) Name() string { return "ollama" }

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalDuration       int64 `json:"eval_duration"`
}

func (o *Ollama) Complete(ctx context.Context, req *Request) (*Response, error) {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	body := ollamaRequest{
		Model:    req.Model,
		Messages: msgs,
		Stream:   false,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(raw))
	}

	var oResp ollamaResponse
	if err := json.Unmarshal(raw, &oResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	promptTokens := oResp.PromptEvalCount
	completionTokens := oResp.EvalCount

	return &Response{
		Content: oResp.Message.Content,
		Usage: models.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
		Model:   req.Model,
		Latency: time.Since(start),
	}, nil
}
