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
)

const providerAnthropic = "anthropic"

// Anthropic implements Provider for the Anthropic Messages API.
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropic creates an Anthropic provider.
func NewAnthropic(cfg ProviderConfig) *Anthropic {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	return &Anthropic{
		apiKey:  cfg.APIKey,
		baseURL: base,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns the provider name.
func (a *Anthropic) Name() string { return providerAnthropic }

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Stop        []string           `json:"stop_sequences,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete sends a prompt to the Anthropic API and returns the response.
func (a *Anthropic) Complete(ctx context.Context, req *Request) (*Response, error) {
	var systemPrompt string
	var msgs []anthropicMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			msgs = append(msgs, anthropicMessage(m))
		}
	}

	body := anthropicRequest{
		Model:     req.Model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
		System:    systemPrompt,
		Stop:      req.Stop,
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", providerAnthropic, err)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set(providerAnthropic+"-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", providerAnthropic, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s error (status %d): %s", providerAnthropic, resp.StatusCode, string(raw))
	}

	var aResp anthropicResponse
	if err := json.Unmarshal(raw, &aResp); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", providerAnthropic, err)
	}

	var sb strings.Builder
	for _, c := range aResp.Content {
		sb.WriteString(c.Text)
	}

	totalTokens := aResp.Usage.InputTokens + aResp.Usage.OutputTokens
	return &Response{
		Content: sb.String(),
		Usage: Usage{
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		},
		Model:      req.Model,
		StopReason: aResp.StopReason,
		Latency:    time.Since(start),
	}, nil
}
