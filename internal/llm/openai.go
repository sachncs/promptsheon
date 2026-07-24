package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// slowCallLogThreshold is the LLM call latency above which we
// emit a debug log. PERF-LLM-2: ops needs visibility when the
// upstream provider is slow; a 5s threshold is the SLO red
// zone (the p99 alert fires at 5s).
const slowCallLogThreshold = 5 * time.Second

// OpenAI implements Provider for the OpenAI Responses API using the
// official openai-go/v3 SDK. The provider name is "openai".
type OpenAI struct {
	client  openai.Client
	baseURL string
}

// NewOpenAI creates an OpenAI provider. cfg.APIKey is the
// registry-level fallback key; the per-call key
// (PerCallKeyFromContext) overrides it for a single request
// when set. The OpenAI client is constructed once at provider
// creation; per-call requests that need a different key
// construct a transient client on the fly. This is the
// per-prompt / per-workspace key binding the vault exposes.
//
// PERF-LLM-1: pass a tuned http.Transport to the SDK. The
// default transport caps MaxIdleConnsPerHost at 2, which forces
// a new TCP+TLS handshake for almost every concurrent request
// to the upstream. The tuned values here match the
// internal/llm defaults and the LLM gateway's expectations.
func NewOpenAI(cfg ProviderConfig) *OpenAI {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com"
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
	return &OpenAI{
		client:  openai.NewClient(opts...),
		baseURL: base,
	}
}

// clientFor returns the SDK client authenticated with the
// per-call key when set, or the registry-level client
// otherwise. The transient client is built on every call;
// v3 client construction is allocation-light (one HTTP
// transport) and the per-call key must not be cached in the
// receiver for security reasons.
func (o *OpenAI) clientFor(ctx context.Context) openai.Client {
	if k := PerCallKeyFromContext(ctx); k != "" {
		opts := []option.RequestOption{
			option.WithBaseURL(o.baseURL),
			option.WithAPIKey(k),
			option.WithHTTPClient(&http.Client{
				Transport: tunedTransport(),
				Timeout:   60 * time.Second,
			}),
		}
		return openai.NewClient(opts...)
	}
	return o.client
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
	c := o.clientFor(ctx)
	resp, err := c.Responses.New(ctx, params)
	duration := time.Since(start)
	// PERF-LLM-2: log slow calls so ops can correlate upstream
	// latency with the SLO alert dashboard.
	if duration > slowCallLogThreshold {
		slog.Debug("slow LLM call",
			"provider", o.Name(),
			"model", req.Model,
			"duration", duration,
			"err", err,
		)
	}
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
		Latency:    duration,
	}, nil
}
