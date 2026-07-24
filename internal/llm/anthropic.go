package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic implements Provider for the Anthropic Messages API using
// the official anthropic-sdk-go.
type Anthropic struct {
	client  anthropic.Client
	baseURL string
}

// NewAnthropic creates an Anthropic provider. cfg.APIKey is required;
// cfg.BaseURL defaults to https://api.anthropic.com when empty.
//
// PERF-LLM-1: tuned http.Transport (shared with OpenAI).
func NewAnthropic(cfg ProviderConfig) *Anthropic {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	opts := []option.RequestOption{
		option.WithBaseURL(base),
		option.WithHTTPClient(&http.Client{
			Transport: tunedTransport(),
			Timeout:   60 * time.Second,
		}),
	}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	return &Anthropic{
		client:  anthropic.NewClient(opts...),
		baseURL: base,
	}
}

// Name returns the provider name.
func (a *Anthropic) Name() string { return string(ProviderAnthropic) }

// Anthropic satisfies the Provider interface.
var _ Provider = (*Anthropic)(nil)

// Complete sends a prompt to the Anthropic API and returns the response.
func (a *Anthropic) Complete(ctx context.Context, req *Request) (*Response, error) {
	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	var systemBlocks []anthropic.TextBlockParam
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: m.Content})
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		default:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}
	if len(req.Stop) > 0 {
		params.StopSequences = req.Stop
	}

	start := time.Now()
	msg, err := a.client.Messages.New(ctx, params)
	duration := time.Since(start)
	// PERF-LLM-2: log slow calls.
	if duration > slowCallLogThreshold {
		slog.Debug("slow LLM call",
			"provider", a.Name(),
			"model", req.Model,
			"duration", duration,
			"err", err,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", ProviderAnthropic, err)
	}

	var content string
	for _, block := range msg.Content {
		if text := block.Text; text != "" {
			content += text
		}
	}

	inTok, outTok := int64(msg.Usage.InputTokens), int64(msg.Usage.OutputTokens)

	return &Response{
		Content: content,
		Usage: Usage{
			PromptTokens:     int(inTok),
			CompletionTokens: int(outTok),
			TotalTokens:      int(inTok + outTok),
		},
		Model:      string(msg.Model),
		StopReason: string(msg.StopReason),
		Latency:    duration,
	}, nil
}
