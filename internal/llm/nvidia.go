package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NvidiaProvider implements the NVIDIA NIM API provider.
type NvidiaProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	config     ProviderConfig
}

// NewNvidia creates a new NVIDIA NIM provider.
func NewNvidia(cfg ProviderConfig) *NvidiaProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://integrate.api.nvidia.com/v1"
	}
	return &NvidiaProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		config: cfg,
	}
}

// Name returns the provider name.
func (p *NvidiaProvider) Name() string {
	return "nvidia"
}

// nvidiaRequest represents the request body for NVIDIA NIM API.
type nvidiaRequest struct {
	Model       string           `json:"model"`
	Messages    []nvidiaMessage  `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	ExtraBody   *nvidiaExtraBody `json:"extra_body,omitempty"`
}

// nvidiaExtraBody represents extra body parameters for NVIDIA NIM.
type nvidiaExtraBody struct {
	ChatTemplateKwargs *chatTemplateKwargs `json:"chat_template_kwargs,omitempty"`
	ReasoningBudget    int                 `json:"reasoning_budget,omitempty"`
}

// chatTemplateKwargs represents chat template parameters.
type chatTemplateKwargs struct {
	EnableThinking bool `json:"enable_thinking"`
}

// nvidiaMessage represents a message in NVIDIA NIM API.
type nvidiaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// nvidiaResponse represents the response from NVIDIA NIM API.
type nvidiaResponse struct {
	ID      string         `json:"id"`
	Choices []nvidiaChoice `json:"choices"`
	Usage   nvidiaUsage    `json:"usage"`
}

// nvidiaChoice represents a choice in the response.
type nvidiaChoice struct {
	Index        int           `json:"index"`
	Message      nvidiaMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// nvidiaUsage represents token usage.
type nvidiaUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// nvidiaStreamChunk represents a streaming chunk.
type nvidiaStreamChunk struct {
	ID      string               `json:"id"`
	Choices []nvidiaStreamChoice `json:"choices"`
}

// nvidiaStreamChoice represents a choice in streaming.
type nvidiaStreamChoice struct {
	Index        int               `json:"index"`
	Delta        nvidiaStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}

// nvidiaStreamDelta represents a delta in streaming.
type nvidiaStreamDelta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// Complete sends a request to the NVIDIA NIM API.
func (p *NvidiaProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	// Build messages
	messages := make([]nvidiaMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = nvidiaMessage(msg)
	}

	// Build request body
	body := nvidiaRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	// Add reasoning support for Nemotron models. M-10 fix: the
	// previous implementation always enabled thinking + set the
	// reasoning budget to max_tokens for any model whose name
	// contains "nemotron", regardless of whether the caller asked
	// for it. This was undocumented and forced a non-trivial
	// behaviour change on every Nemotron request. Make it opt-in
	// via the provider's Extra config: a caller sets
	// Extra["enable_thinking"]="true" and the budget becomes
	// max_tokens; otherwise the body is unchanged.
	if strings.Contains(req.Model, "nemotron") && p.config.Extra["enable_thinking"] == "true" {
		body.ExtraBody = &nvidiaExtraBody{
			ChatTemplateKwargs: &chatTemplateKwargs{
				EnableThinking: true,
			},
			ReasoningBudget: req.MaxTokens,
		}
	}

	// Marshal request
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("nvidia request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle streaming
	if req.Stream {
		return p.handleStream(resp, req.Model, start), nil
	}

	// Read response
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nvidia error (status %d): %s", resp.StatusCode, string(raw))
	}

	// Parse response
	var nResp nvidiaResponse
	if err := json.Unmarshal(raw, &nResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content
	content := ""
	if len(nResp.Choices) > 0 {
		content = nResp.Choices[0].Message.Content
		// Note: Reasoning content would be in a separate field if using streaming
	}

	return &Response{
		Content: content,
		Usage: Usage{
			PromptTokens:     nResp.Usage.PromptTokens,
			CompletionTokens: nResp.Usage.CompletionTokens,
			TotalTokens:      nResp.Usage.TotalTokens,
		},
		Model:      req.Model,
		StopReason: "stop",
		Latency:    time.Since(start),
	}, nil
}

// handleStream handles streaming responses.
func (p *NvidiaProvider) handleStream(resp *http.Response, model string, start time.Time) *Response {
	scanner := bufio.NewScanner(resp.Body)
	var content strings.Builder
	var reasoning strings.Builder
	var ttft time.Duration
	firstToken := true
	var usage Usage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		if firstToken {
			ttft = time.Since(start)
			firstToken = false
		}

		var chunk nvidiaStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.ReasoningContent != "" {
				reasoning.WriteString(delta.ReasoningContent)
			}
			if delta.Content != "" {
				content.WriteString(delta.Content)
			}
		}
	}

	// Combine reasoning and content
	fullContent := content.String()
	if reasoning.Len() > 0 {
		fullContent = "[Reasoning] " + reasoning.String() + "\n\n" + fullContent
	}

	return &Response{
		Content:          fullContent,
		Usage:            usage,
		Model:            model,
		Latency:          time.Since(start),
		TimeToFirstToken: ttft,
	}
}
