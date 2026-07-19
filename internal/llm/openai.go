package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// OpenAI implements Provider for the OpenAI Responses API using the
// official openai-go/v3 SDK. The provider name is "openai".
type OpenAI struct {
	client  openai.Client
	baseURL string
}

// NewOpenAI creates an OpenAI provider. cfg.APIKey is required;
// cfg.BaseURL defaults to https://api.openai.com when empty.
func NewOpenAI(cfg ProviderConfig) *OpenAI {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com"
	}
	opts := []option.RequestOption{option.WithBaseURL(base)}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	return &OpenAI{
		client:  openai.NewClient(opts...),
		baseURL: base,
	}
}

// Name returns the provider name.
func (o *OpenAI) Name() string { return ProviderOpenAI.String() }

// OpenAI satisfies the Provider interface.
var _ Provider = (*OpenAI)(nil)

// Complete sends a prompt to the OpenAI Responses API and returns
// the response. The previous implementation flattened every
// message into a single input string, which collapsed system,
// user, and assistant roles into a single user turn. The new path
// preserves the role of each message by:
//   - prepending a literal "[SYSTEM]\n" / "[USER]\n" / "[ASSISTANT]\n"
//     marker to each block (the v3 Responses API does not
//     directly expose role-tagged input helpers in this binding),
//   - joining with blank lines, and
//   - preserving TopP (previously dropped on the floor).
//
// req.Stop is still not surfaced through the Responses API in v3;
// the parameter is silently dropped. Callers that need
// deterministic truncation should set max_tokens instead.
func (o *OpenAI) Complete(ctx context.Context, req *Request) (*Response, error) {
	var input string
	for _, m := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "" {
			role = "user"
		}
		if input != "" {
			input += "\n\n"
		}
		input += "[" + strings.ToUpper(role) + "]\n" + m.Content
	}

	maxTokens := int64(req.MaxTokens)
	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input),
		},
		Model: req.Model,
	}
	if maxTokens > 0 {
		params.MaxOutputTokens = openai.Int(maxTokens)
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}
	if req.TopP > 0 {
		params.TopP = openai.Float(req.TopP)
	}
	// req.Stop is not surfaced through the Responses API in v3; the
	// parameter is silently dropped. Callers that need deterministic
	// truncation should set max_tokens instead.

	start := time.Now()
	resp, err := o.client.Responses.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", ProviderOpenAI, err)
	}

	inTok, outTok := int64(resp.Usage.InputTokens), int64(resp.Usage.OutputTokens)
	return &Response{
		Content: resp.OutputText(),
		Usage: Usage{
			PromptTokens:     int(inTok),
			CompletionTokens: int(outTok),
			TotalTokens:      int(inTok + outTok),
		},
		Model:      string(resp.Model),
		StopReason: string(resp.Status),
		Latency:    time.Since(start),
	}, nil
}
