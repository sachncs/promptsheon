package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"promptsheon/internal/models"
)

// Azure implements Provider for Azure OpenAI's deployment-based endpoint.
type Azure struct {
	apiKey     string
	resource   string // e.g. "myresource.openai.azure.com"
	deployment string // e.g. "gpt-4"
	apiVersion string // e.g. "2024-02-15-preview"
	client     *http.Client
}

// NewAzure creates an Azure OpenAI provider.
func NewAzure(cfg ProviderConfig) *Azure {
	apiVersion := cfg.Extra["api_version"]
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}
	deployment := cfg.Extra["deployment"]
	if deployment == "" {
		deployment = "gpt-4"
	}
	return &Azure{
		apiKey:     cfg.APIKey,
		resource:   cfg.BaseURL,
		deployment: deployment,
		apiVersion: apiVersion,
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *Azure) Name() string { return "azure" }

type azureRequest struct {
	Model       string          `json:"model,omitempty"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream"`
}

type azureResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta *struct {
			Content string `json:"content"`
		} `json:"delta,omitempty"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (a *Azure) Complete(ctx context.Context, req *Request) (*Response, error) {
	body := azureRequest{
		Messages:  toOpenAIMessages(req.Messages),
		MaxTokens: req.MaxTokens,
		Stop:      req.Stop,
		Stream:    false,
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal azure request: %w", err)
	}

	var url string
	if strings.HasPrefix(a.resource, "http://") || strings.HasPrefix(a.resource, "https://") {
		url = fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			a.resource, a.deployment, a.apiVersion)
	} else {
		url = fmt.Sprintf("https://%s/openai/deployments/%s/chat/completions?api-version=%s",
			a.resource, a.deployment, a.apiVersion)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("azure request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure error (status %d): %s", resp.StatusCode, string(raw))
	}

	var aResp azureResponse
	if err := json.Unmarshal(raw, &aResp); err != nil {
		return nil, fmt.Errorf("decode azure response: %w", err)
	}

	if len(aResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in azure response")
	}

	content := aResp.Choices[0].Message.Content
	totalTokens := aResp.Usage.PromptTokens + aResp.Usage.CompletionTokens

	var stopReason string
	if aResp.Choices[0].FinishReason != nil {
		stopReason = *aResp.Choices[0].FinishReason
	}

	return &Response{
		Content: content,
		Usage: models.Usage{
			PromptTokens:     aResp.Usage.PromptTokens,
			CompletionTokens: aResp.Usage.CompletionTokens,
			TotalTokens:      totalTokens,
		},
		Model:      req.Model,
		StopReason: stopReason,
		Latency:    time.Since(start),
	}, nil
}

func toOpenAIMessages(msgs []Message) []openaiMessage {
	out := make([]openaiMessage, len(msgs))
	for i, m := range msgs {
		out[i] = openaiMessage(m)
	}
	return out
}
