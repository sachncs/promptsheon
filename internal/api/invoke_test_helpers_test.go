package api

// invoke_test_helpers.go wires a production-shaped invoke.Invoker
// backed by deterministic in-memory LLM providers. Tests that need
// to exercise POST /versions/{id}/executions or POST /releases/{id}/invoke
// call newInvokeTestServer; the helper constructs every dependency
// the production wiring would set up (provider registry, executor,
// invoker) without any HTTP I/O.
//
// The original test fixture (newTestServer) built a Server without
// an Invoker, so handlers fell through a stub that persisted a
// zero-token Execution row. That stub was misleading — an audit
// consumer saw invocations that never ran. The new helper makes
// the invoke path real: every request hits an in-memory provider,
// returns a deterministic response with real token counts, and
// surfaces those in the audit chain and the Execution record.

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/testutil"
)

// inMemoryProvider is a deterministic llm.Provider used by the
// test helpers. It echoes the first user message back as the
// assistant content; tests can assert on the round-trip without
// any HTTP I/O. Token counts and cost are fixed (1 prompt / 1
// completion token / $0.01 cost) so audit numbers are predictable.
type inMemoryProvider struct {
	name string
}

func (p *inMemoryProvider) Name() string { return p.name }

func (p *inMemoryProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("inMemoryProvider: model required")
	}
	var echo string
	for _, m := range req.Messages {
		if m.Role == "user" {
			echo = m.Content
			break
		}
	}
	if echo == "" {
		echo = "ok"
	}
	return &llm.Response{
		Content:    echo,
		Model:      req.Model,
		StopReason: "stop",
		Usage: llm.Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
	}, nil
}

// passthroughEnforcer satisfies invoke.Enforcer without enforcing
// any budget or quota. Tests use it to drive the Invoker without
// touching the budget/quota stores.
type passthroughEnforcer struct{}

func (passthroughEnforcer) EnforceBudget(_ context.Context, _ string, _ float64) error {
	return nil
}
func (passthroughEnforcer) EnforceQuota(_ context.Context, _ string) error {
	return nil
}

// newInvokeTestServer wires a real invoke.Invoker backed by an
// in-memory LLM provider and returns a Server with the invoker
// attached via WithInvoker. The wiring mirrors production:
//
//	provider registry  ->  executor.Caller
//	executor.Executor  ->  invoke.Invoker
//	invoke.Invoker     ->  api.Server.WithInvoker
//
// Every Complete call hits the in-memory provider, returns a
// deterministic response with real token counts, and surfaces
// those in the audit chain and the Execution record. No HTTP I/O.
func newInvokeTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	return newInvokeTestServerWithRepo(t, newMockRepo(), opts...)
}

// newInvokeTestServerWithRepo is the repo-injecting form. Tests
// that need a pre-populated store (release fixtures, capability
// fixtures) call this so the server sees the seeded rows.
// inMemoryArtifactLoader returns the same canned JSON for every
// artifact kind, so the release Resolver can populate
// ModelPolicy.Provider + ModelPolicy.Model without a real CAS.
// The prompt content and runtime-policy defaults are also
// canned; tests that need to assert on prompt contents can
// wrap this loader.
type inMemoryArtifactLoader struct{}

func (inMemoryArtifactLoader) Load(ctx context.Context, kind capability.ArtifactKind, hash string) ([]byte, error) {
	switch kind {
	case capability.ArtifactPrompt:
		return []byte("test prompt"), nil
	case capability.ArtifactModelPolicy:
		return []byte(`{"provider":"openai","model":"gpt-4","revision":"test"}`), nil
	case capability.ArtifactRuntimePolicy:
		return []byte(`{"max_output_tokens":1024,"temperature":0.7}`), nil
	}
	return nil, fmt.Errorf("inMemoryArtifactLoader: unknown kind %q", kind)
}

func newInvokeTestServerWithRepo(t *testing.T, repo *mockRepo, opts ...Option) *Server {
	t.Helper()

	// Wire a real release.Resolver backed by the in-memory
	// ArtifactLoader so /releases/{id}/invoke resolves
	// Provider+Model from the Manifest (mirroring production).
	resolver := release.NewResolver(repo, inMemoryArtifactLoader{})

	providers := llm.NewRegistry()
	openai := &inMemoryProvider{name: "openai"}
	anthropic := &inMemoryProvider{name: "anthropic"}
	stub := &inMemoryProvider{name: "stub"}
	providers.Register("openai", func(_ llm.ProviderConfig) llm.Provider { return openai })
	providers.Register("anthropic", func(_ llm.ProviderConfig) llm.Provider { return anthropic })
	providers.Register("stub", func(_ llm.ProviderConfig) llm.Provider { return stub })
	providers.Configure("openai", llm.ProviderConfig{APIKey: "sk-test"})
	providers.Configure("anthropic", llm.ProviderConfig{APIKey: "sk-test"})
	providers.Configure("stub", llm.ProviderConfig{APIKey: "sk-test"})

	bus := eventbus.NewMemory()
	caller := executor.Caller(func(ctx context.Context, req executor.InvokeRequest) (executor.InvokeResult, error) {
		if req.Provider == "" {
			return executor.InvokeResult{Status: "error", Error: "no provider specified"}, executor.ErrProviderMissing
		}
		p, err := providers.Get(req.Provider)
		if err != nil {
			return executor.InvokeResult{Status: "error", Error: "provider not registered: " + req.Provider}, executor.ErrProviderMissing
		}
		llmReq := &llm.Request{
			Model: req.Model,
			Messages: []llm.Message{
				{Role: "user", Content: string(req.Input)},
			},
			MaxTokens: 1024,
		}
		start := time.Now()
		resp, err := p.Complete(ctx, llmReq)
		if err != nil {
			return executor.InvokeResult{
				Status:    "error",
				Error:     err.Error(),
				LatencyMS: time.Since(start).Milliseconds(),
			}, err
		}
		return executor.InvokeResult{
			Output:       []byte(resp.Content),
			PromptTokens: resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			Status:       "ok",
			CostUSDMicro: 10_000,
			LatencyMS:    time.Since(start).Milliseconds(),
		}, nil
	})

	exec := executor.New(bus, caller)
	agg := observation.NewAggregator(nil)
	enforcer := passthroughEnforcer{}
	inv := invoke.New(enforcer, agg, exec)

	collector := metrics.NewCollector()
	inv.WithObservability(
		collector,
		nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	)

	defaults := []Option{
		WithProviders(providers),
		WithInvoker(inv),
		WithCollector(collector),
		WithReleaseResolver(resolver),
	}
	allOpts := append(defaults, opts...)

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(newRepositories(repo), logger, allOpts...)
}

// silence the unused import linter when we trim imports later.
var _ = context.Background

// NewTestServer is the TEST-INFRA-2 canonical entry point for
// tests that need an *api.Server. It pairs with internal/testutil
// .NewTestDB: the helper here wraps the api.Server construction
// so the test layer can be found in one place (internal/testutil
// for stores, internal/api for the server). The repo is
// automatically closed via t.Cleanup.
//
// Callers that need a custom repo (e.g. seeded release fixtures)
// use newInvokeTestServerWithRepo above; the no-arg form is
// for the common case.
func NewTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := testutil.DiscardLogger()
	defaults := []Option{WithProviders(llm.NewRegistry())}
	allOpts := append(defaults, opts...)
	s := NewServer(store.NewRepositories(db), logger, allOpts...)
	t.Cleanup(func() { _ = db.Close() })
	return s
}
