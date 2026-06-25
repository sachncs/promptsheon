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
	"sync/atomic"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
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

// keyForRequest returns the API key to use for a request. Per-call keys
// (set via WithPerCallKey) override the provider's default. This is
// what enables prompt-level provider bindings stored in the vault to
// take effect.
func (o *OpenAI) keyForRequest(ctx context.Context) string {
	if k := PerCallKeyFromContext(ctx); k != "" {
		return k
	}
	return o.apiKey
}

// Stream sends a streaming request to OpenAI and returns the response body as a reader.
// M-11 fix: the returned ReadCloser is now a thin wrapper that
// documents the close contract and avoids the previous surprise
// where the caller had to know to call Close to release the
// connection. The wrapper's Close method delegates to the
// underlying body and is safe to call multiple times.
func (o *OpenAI) Stream(ctx context.Context, req *Request) (io.ReadCloser, error) {
	messages := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openaiMessage(m)
	}

	body := openaiRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    true,
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}
	if len(req.Stop) > 0 {
		body.Stop = req.Stop
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.keyForRequest(ctx))

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			return nil, fmt.Errorf("openai error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("openai error (%d)", resp.StatusCode)
	}

	return &streamCloser{ReadCloser: resp.Body}, nil
}

// streamCloser is a thin wrapper that makes the close contract
// explicit and idempotent. The previous implementation returned
// the raw http.Response.Body, which the caller was expected to
// Close but the type did not advertise that obligation.
type streamCloser struct {
	io.ReadCloser
	closed atomic.Bool
}

func (s *streamCloser) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	return s.ReadCloser.Close()
}

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
		msgs[i] = openaiMessage(m)
	}

	body := openaiRequest{
		Model:     req.Model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
		Stop:      req.Stop,
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
	httpReq.Header.Set("Authorization", "Bearer "+o.keyForRequest(ctx))

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if req.Stream {
		return o.handleStream(ctx, resp, req.Model, start)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
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

func (o *OpenAI) handleStream(ctx context.Context, resp *http.Response, model string, start time.Time) (*Response, error) {
	// Use a generously-sized scanner buffer; SSE event lines can be
	// much larger than the default 64KB token.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var content strings.Builder
	var ttft time.Duration
	firstToken := true
	var usage models.Usage

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
