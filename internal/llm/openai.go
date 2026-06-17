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

	"promptsheon/internal/models"
)

// OpenAI implements Provider for the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates an OpenAI provider.
func NewOpenAI(cfg ProviderConfig) *OpenAI {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &OpenAI{
		apiKey:  cfg.APIKey,
		baseURL: base,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAI) Name() string { return "openai" }

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (o *OpenAI) Complete(ctx context.Context, req *Request) (*Response, error) {
	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}

	body := openaiRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Stop:        req.Stop,
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}
	if req.Stream {
		body.Stream = true
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if req.Stream {
		return o.handleStream(resp, req.Model, start)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai error (status %d): %s", resp.StatusCode, string(raw))
	}

	var oResp openaiResponse
	if err := json.Unmarshal(raw, &oResp); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	content := ""
	var finishReason string
	if len(oResp.Choices) > 0 {
		content = oResp.Choices[0].Message.Content
		finishReason = oResp.Choices[0].FinishReason
	}

	return &Response{
		Content: content,
		Usage: models.Usage{
			PromptTokens:     oResp.Usage.PromptTokens,
			CompletionTokens: oResp.Usage.CompletionTokens,
			TotalTokens:      oResp.Usage.TotalTokens,
		},
		Model:      req.Model,
		StopReason: finishReason,
		Latency:    time.Since(start),
	}, nil
}

type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (o *OpenAI) handleStream(resp *http.Response, model string, start time.Time) (*Response, error) {
	scanner := bufio.NewScanner(resp.Body)
	var content strings.Builder
	var ttft time.Duration
	firstToken := true
	var usage models.Usage

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

		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}
	}

	return &Response{
		Content:          content.String(),
		Usage:            usage,
		Model:            model,
		Latency:          time.Since(start),
		TimeToFirstToken: ttft,
	}, nil
}
