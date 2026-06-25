package llm

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

// CallMetrics holds the observed metrics for a single LLM call.
type CallMetrics struct {
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Latency  time.Duration `json:"latency"`
	Usage    models.Usage  `json:"usage"`
	CostUSD  float64       `json:"cost_usd"`
	Error    string        `json:"error,omitempty"`
}

// MetricsCollector is a callback invoked after every Complete call.
type MetricsCollector func(CallMetrics)

// Instrumented wraps a Provider and records metrics after each call.
type Instrumented struct {
	inner     Provider
	collector MetricsCollector
	logger    *slog.Logger
}

// NewInstrumented wraps a provider with metrics collection.
func NewInstrumented(p Provider, collector MetricsCollector, logger *slog.Logger) *Instrumented {
	return &Instrumented{inner: p, collector: collector, logger: logger}
}

func (i *Instrumented) Name() string { return i.inner.Name() }

// Complete delegates to the inner provider, then records metrics.
func (i *Instrumented) Complete(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()
	resp, err := i.inner.Complete(ctx, req)
	latency := time.Since(start)

	metrics := CallMetrics{
		Provider: i.inner.Name(),
		Model:    req.Model,
		Latency:  latency,
	}

	if err != nil {
		metrics.Error = err.Error()
	} else {
		metrics.Usage = resp.Usage
		metrics.CostUSD = CalculateCost(req.Model, resp.Usage)
	}

	if i.collector != nil {
		i.collector(metrics)
	}
	if i.logger != nil {
		i.logger.Debug("llm call",
			"provider", metrics.Provider,
			"model", metrics.Model,
			"latency_ms", latency.Milliseconds(),
			"prompt_tokens", metrics.Usage.PromptTokens,
			"completion_tokens", metrics.Usage.CompletionTokens,
			"cost_usd", metrics.CostUSD,
		)
	}

	return resp, err
}

// AggregateMetrics holds cumulative stats across many calls.
type AggregateMetrics struct {
	mu           sync.Mutex
	TotalCalls   int                         `json:"total_calls"`
	TotalTokens  int                         `json:"total_tokens"`
	TotalCostUSD float64                     `json:"total_cost_usd"`
	TotalLatency time.Duration               `json:"total_latency"`
	ByProvider   map[string]*ProviderMetrics `json:"by_provider"`
	ByModel      map[string]*ProviderMetrics `json:"by_model"`
}

// ProviderMetrics holds aggregated metrics for a single provider or model.
type ProviderMetrics struct {
	Calls      int           `json:"calls"`
	Tokens     int           `json:"tokens"`
	CostUSD    float64       `json:"cost_usd"`
	Latency    time.Duration `json:"latency"`
	AvgLatency time.Duration `json:"avg_latency"`
}

// NewAggregateMetrics creates an empty AggregateMetrics.
func NewAggregateMetrics() *AggregateMetrics {
	return &AggregateMetrics{
		ByProvider: make(map[string]*ProviderMetrics),
		ByModel:    make(map[string]*ProviderMetrics),
	}
}

// Collect returns a MetricsCollector that feeds into this aggregator.
func (a *AggregateMetrics) Collect() MetricsCollector {
	return func(m CallMetrics) {
		a.mu.Lock()
		defer a.mu.Unlock()

		a.TotalCalls++
		a.TotalTokens += m.Usage.TotalTokens
		a.TotalCostUSD += m.CostUSD
		a.TotalLatency += m.Latency

		// By provider
		pm, ok := a.ByProvider[m.Provider]
		if !ok {
			pm = &ProviderMetrics{}
			a.ByProvider[m.Provider] = pm
		}
		pm.Calls++
		pm.Tokens += m.Usage.TotalTokens
		pm.CostUSD += m.CostUSD
		pm.Latency += m.Latency
		pm.AvgLatency = pm.Latency / time.Duration(pm.Calls)

		// By model
		mm, ok := a.ByModel[m.Model]
		if !ok {
			mm = &ProviderMetrics{}
			a.ByModel[m.Model] = mm
		}
		mm.Calls++
		mm.Tokens += m.Usage.TotalTokens
		mm.CostUSD += m.CostUSD
		mm.Latency += m.Latency
		mm.AvgLatency = mm.Latency / time.Duration(mm.Calls)
	}
}
