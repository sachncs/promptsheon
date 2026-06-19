// Package metrics provides Prometheus-compatible metrics collection and
// export for the Promptsheon platform.
package metrics

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Counter is a monotonically increasing metric.
type Counter struct {
	mu     sync.Mutex
	value  float64
	labels map[string]string
}

func newCounter(labels map[string]string) *Counter {
	return &Counter{labels: labels}
}

func (c *Counter) Inc()           { c.Add(1) }
func (c *Counter) Add(v float64)  { c.mu.Lock(); c.value += v; c.mu.Unlock() }
func (c *Counter) Value() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.value }

// Histogram tracks distribution of values.
type Histogram struct {
	mu      sync.Mutex
	count   int64
	sum     float64
	buckets []float64
	counts  []int64
	labels  map[string]string
	// For percentile calculation (capped to prevent unbounded growth)
	values []float64
}

const maxHistogramValues = 10000

func newHistogram(labels map[string]string) *Histogram {
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}
	return &Histogram{
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1),
		labels:  labels,
	}
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += v
	// Cap values to prevent unbounded memory growth
	if len(h.values) >= maxHistogramValues {
		h.values = h.values[1:]
	}
	h.values = append(h.values, v)
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.buckets)]++
}

func (h *Histogram) Count() int64 { h.mu.Lock(); defer h.mu.Unlock(); return h.count }
func (h *Histogram) Sum() float64 { h.mu.Lock(); defer h.mu.Unlock(); return h.sum }

// Avg returns the mean of all observed values.
func (h *Histogram) Avg() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

// Percentile returns the p-th percentile (0-100) of observed values.
func (h *Histogram) Percentile(p float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.values) == 0 {
		return 0
	}
	sorted := make([]float64, len(h.values))
	copy(sorted, h.values)
	sort.Float64s(sorted)
	idx := (p / 100.0) * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// P50 returns the 50th percentile (median).
func (h *Histogram) P50() float64 { return h.Percentile(50) }

// P95 returns the 95th percentile.
func (h *Histogram) P95() float64 { return h.Percentile(95) }

// P99 returns the 99th percentile.
func (h *Histogram) P99() float64 { return h.Percentile(99) }

// Collector holds all metrics for the application.
type Collector struct {
	// API metrics
	RequestsTotal   *Counter
	RequestDuration *Histogram
	ErrorsTotal     *Counter

	// LLM metrics
	LLMCallsTotal   *Counter
	LLMLatency      *Histogram
	LLMTokensTotal  *Counter
	LLMInputTokens  *Counter
	LLMOutputTokens *Counter
	LLMCostUSD      *Counter
	LLMTTFT         *Histogram

	// Eval metrics
	EvalRunsTotal  *Counter
	EvalCasesTotal *Counter
	EvalDuration   *Histogram

	// Workflow metrics
	WorkflowRunsTotal *Counter
	WorkflowDuration  *Histogram
	WorkflowActive    *Gauge

	// Review metrics
	ReviewPendingCount  *Gauge
	ReviewTotalCount    *Counter
	ReviewApprovedCount *Counter
	ReviewRejectedCount *Counter
	ReviewDuration      *Histogram

	// Guardrail metrics
	GuardrailViolations *Counter
	GuardrailBlocks     *Counter
	GuardrailPasses     *Counter

	// Agent execution metrics
	AgentExecutionsTotal  *Counter
	AgentExecutionLatency *Histogram

	// Hallucination score histogram
	HallucinationScores *Histogram
}

// Gauge is a metric that can go up and down.
type Gauge struct {
	mu    sync.Mutex
	value float64
}

func (g *Gauge) Set(v float64)  { g.mu.Lock(); g.value = v; g.mu.Unlock() }
func (g *Gauge) Inc()           { g.mu.Lock(); g.value++; g.mu.Unlock() }
func (g *Gauge) Dec()           { g.mu.Lock(); g.value--; g.mu.Unlock() }
func (g *Gauge) Value() float64 { g.mu.Lock(); defer g.mu.Unlock(); return g.value }

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		RequestsTotal:         newCounter(nil),
		RequestDuration:       newHistogram(nil),
		ErrorsTotal:           newCounter(nil),
		LLMCallsTotal:         newCounter(nil),
		LLMLatency:            newHistogram(nil),
		LLMTokensTotal:        newCounter(nil),
		LLMInputTokens:        newCounter(nil),
		LLMOutputTokens:       newCounter(nil),
		LLMCostUSD:            newCounter(nil),
		LLMTTFT:               newHistogram(nil),
		EvalRunsTotal:         newCounter(nil),
		EvalCasesTotal:        newCounter(nil),
		EvalDuration:          newHistogram(nil),
		WorkflowRunsTotal:     newCounter(nil),
		WorkflowDuration:      newHistogram(nil),
		WorkflowActive:        &Gauge{},
		ReviewPendingCount:    &Gauge{},
		ReviewTotalCount:      newCounter(nil),
		ReviewApprovedCount:   newCounter(nil),
		ReviewRejectedCount:   newCounter(nil),
		ReviewDuration:        newHistogram(nil),
		GuardrailViolations:   newCounter(nil),
		GuardrailBlocks:       newCounter(nil),
		GuardrailPasses:       newCounter(nil),
		AgentExecutionsTotal:  newCounter(nil),
		AgentExecutionLatency: newHistogram(nil),
		HallucinationScores:   newHistogram(nil),
	}
}

// Handler returns an http.Handler that serves Prometheus exposition format.
func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, c.prometheusFormat())
	})
}

// Summary holds aggregated metrics for the dashboard API.
type Summary struct {
	APIMetrics struct {
		TotalRequests int64   `json:"total_requests"`
		AvgLatencyMs  float64 `json:"avg_latency_ms"`
		P50LatencyMs  float64 `json:"p50_latency_ms"`
		P95LatencyMs  float64 `json:"p95_latency_ms"`
		P99LatencyMs  float64 `json:"p99_latency_ms"`
		TotalErrors   int64   `json:"total_errors"`
		ErrorRate     float64 `json:"error_rate"`
	} `json:"api_metrics"`
	LLMMetrics struct {
		TotalCalls   int64   `json:"total_calls"`
		TotalTokens  int64   `json:"total_tokens"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		AvgLatencyMs float64 `json:"avg_latency_ms"`
		P95LatencyMs float64 `json:"p95_latency_ms"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	} `json:"llm_metrics"`
	EvalMetrics struct {
		TotalRuns   int64   `json:"total_runs"`
		TotalCases  int64   `json:"total_cases"`
		AvgDuration float64 `json:"avg_duration_ms"`
	} `json:"eval_metrics"`
	WorkflowMetrics struct {
		TotalRuns   int64   `json:"total_runs"`
		ActiveCount float64 `json:"active_count"`
		AvgDuration float64 `json:"avg_duration_ms"`
	} `json:"workflow_metrics"`
	ReviewMetrics struct {
		PendingCount  float64 `json:"pending_count"`
		TotalReviews  int64   `json:"total_reviews"`
		ApprovedCount int64   `json:"approved_count"`
		RejectedCount int64   `json:"rejected_count"`
		ApprovalRate  float64 `json:"approval_rate"`
		AvgDurationMs float64 `json:"avg_duration_ms"`
	} `json:"review_metrics"`
	GuardrailMetrics struct {
		Violations int64 `json:"violations"`
		Blocks     int64 `json:"blocks"`
		Passes     int64 `json:"passes"`
	} `json:"guardrail_metrics"`
	HallucinationMetrics struct {
		AvgScore float64 `json:"avg_score"`
		P95Score float64 `json:"p95_score"`
	} `json:"hallucination_metrics"`
	TraceStats any `json:"trace_stats,omitempty"`
}

// GetSummary returns a snapshot of all metrics for the dashboard.
func (c *Collector) GetSummary() *Summary {
	s := &Summary{}
	s.APIMetrics.TotalRequests = int64(c.RequestsTotal.Value())
	s.APIMetrics.AvgLatencyMs = c.RequestDuration.Avg() * 1000
	s.APIMetrics.P50LatencyMs = c.RequestDuration.P50() * 1000
	s.APIMetrics.P95LatencyMs = c.RequestDuration.P95() * 1000
	s.APIMetrics.P99LatencyMs = c.RequestDuration.P99() * 1000
	s.APIMetrics.TotalErrors = int64(c.ErrorsTotal.Value())
	if s.APIMetrics.TotalRequests > 0 {
		s.APIMetrics.ErrorRate = float64(s.APIMetrics.TotalErrors) / float64(s.APIMetrics.TotalRequests) * 100
	}

	s.LLMMetrics.TotalCalls = int64(c.LLMCallsTotal.Value())
	s.LLMMetrics.TotalTokens = int64(c.LLMTokensTotal.Value())
	s.LLMMetrics.InputTokens = int64(c.LLMInputTokens.Value())
	s.LLMMetrics.OutputTokens = int64(c.LLMOutputTokens.Value())
	s.LLMMetrics.AvgLatencyMs = c.LLMLatency.Avg() * 1000
	s.LLMMetrics.P95LatencyMs = c.LLMLatency.P95() * 1000
	s.LLMMetrics.TotalCostUSD = c.LLMCostUSD.Value()
	s.LLMMetrics.AvgTTFTMs = c.LLMTTFT.Avg() * 1000

	s.EvalMetrics.TotalRuns = int64(c.EvalRunsTotal.Value())
	s.EvalMetrics.TotalCases = int64(c.EvalCasesTotal.Value())
	s.EvalMetrics.AvgDuration = c.EvalDuration.Avg() * 1000

	s.WorkflowMetrics.TotalRuns = int64(c.WorkflowRunsTotal.Value())
	s.WorkflowMetrics.ActiveCount = c.WorkflowActive.Value()
	s.WorkflowMetrics.AvgDuration = c.WorkflowDuration.Avg() * 1000

	s.ReviewMetrics.PendingCount = c.ReviewPendingCount.Value()
	s.ReviewMetrics.TotalReviews = int64(c.ReviewTotalCount.Value())
	s.ReviewMetrics.ApprovedCount = int64(c.ReviewApprovedCount.Value())
	s.ReviewMetrics.RejectedCount = int64(c.ReviewRejectedCount.Value())
	if s.ReviewMetrics.TotalReviews > 0 {
		s.ReviewMetrics.ApprovalRate = float64(s.ReviewMetrics.ApprovedCount) / float64(s.ReviewMetrics.TotalReviews) * 100
	}
	s.ReviewMetrics.AvgDurationMs = c.ReviewDuration.Avg() * 1000

	s.GuardrailMetrics.Violations = int64(c.GuardrailViolations.Value())
	s.GuardrailMetrics.Blocks = int64(c.GuardrailBlocks.Value())
	s.GuardrailMetrics.Passes = int64(c.GuardrailPasses.Value())

	s.HallucinationMetrics.AvgScore = c.HallucinationScores.Avg()
	s.HallucinationMetrics.P95Score = c.HallucinationScores.P95()

	return s
}

func (c *Collector) prometheusFormat() string {
	var sb strings.Builder

	writeCounter := func(name, help string, val float64) {
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, help))
		sb.WriteString(fmt.Sprintf("# TYPE %s counter\n", name))
		sb.WriteString(fmt.Sprintf("%s %g\n", name, val))
	}

	writeHistogram := func(name, help string, h *Histogram) {
		h.mu.Lock()
		defer h.mu.Unlock()
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, help))
		sb.WriteString(fmt.Sprintf("# TYPE %s histogram\n", name))
		for i, b := range h.buckets {
			sb.WriteString(fmt.Sprintf("%s_bucket{le=\"%g\"} %d\n", name, b, h.counts[i]))
		}
		sb.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", name, h.counts[len(h.buckets)]))
		sb.WriteString(fmt.Sprintf("%s_sum %g\n", name, h.sum))
		sb.WriteString(fmt.Sprintf("%s_count %d\n", name, h.count))
	}

	writeGauge := func(name, help string, val float64) {
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, help))
		sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", name))
		sb.WriteString(fmt.Sprintf("%s %g\n", name, val))
	}

	writeCounter("promptsheon_http_requests_total", "Total HTTP requests", c.RequestsTotal.Value())
	writeHistogram("promptsheon_http_request_duration_seconds", "HTTP request duration", c.RequestDuration)
	writeCounter("promptsheon_http_errors_total", "Total HTTP errors", c.ErrorsTotal.Value())

	writeCounter("promptsheon_llm_calls_total", "Total LLM calls", c.LLMCallsTotal.Value())
	writeHistogram("promptsheon_llm_latency_seconds", "LLM call latency", c.LLMLatency)
	writeCounter("promptsheon_llm_tokens_total", "Total LLM tokens consumed", c.LLMTokensTotal.Value())
	writeCounter("promptsheon_llm_input_tokens_total", "Total LLM input tokens", c.LLMInputTokens.Value())
	writeCounter("promptsheon_llm_output_tokens_total", "Total LLM output tokens", c.LLMOutputTokens.Value())
	writeCounter("promptsheon_llm_cost_usd_total", "Total LLM cost in USD", c.LLMCostUSD.Value())
	writeHistogram("promptsheon_llm_ttft_seconds", "Time to first token", c.LLMTTFT)

	writeCounter("promptsheon_eval_runs_total", "Total eval runs", c.EvalRunsTotal.Value())
	writeCounter("promptsheon_eval_cases_total", "Total eval cases executed", c.EvalCasesTotal.Value())
	writeHistogram("promptsheon_eval_duration_seconds", "Eval run duration", c.EvalDuration)

	writeCounter("promptsheon_workflow_runs_total", "Total workflow runs", c.WorkflowRunsTotal.Value())
	writeHistogram("promptsheon_workflow_duration_seconds", "Workflow run duration", c.WorkflowDuration)
	writeGauge("promptsheon_workflow_active", "Currently active workflows", c.WorkflowActive.Value())

	writeGauge("promptsheon_review_pending", "Pending reviews", c.ReviewPendingCount.Value())
	writeCounter("promptsheon_review_total", "Total reviews", c.ReviewTotalCount.Value())
	writeCounter("promptsheon_review_approved_total", "Approved reviews", c.ReviewApprovedCount.Value())
	writeCounter("promptsheon_review_rejected_total", "Rejected reviews", c.ReviewRejectedCount.Value())
	writeHistogram("promptsheon_review_duration_seconds", "Review duration", c.ReviewDuration)

	writeCounter("promptsheon_guardrail_violations_total", "Guardrail violations", c.GuardrailViolations.Value())
	writeCounter("promptsheon_guardrail_blocks_total", "Guardrail blocks", c.GuardrailBlocks.Value())
	writeCounter("promptsheon_guardrail_passes_total", "Guardrail passes", c.GuardrailPasses.Value())

	writeHistogram("promptsheon_hallucination_scores", "Hallucination scores", c.HallucinationScores)

	return sb.String()
}

// SortLabels returns a deterministically sorted string of label key=value pairs.
func SortLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	return strings.Join(parts, ",")
}

// DurationMs converts a time.Duration to milliseconds as float64.
func DurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// DurationSec converts a time.Duration to seconds as float64.
func DurationSec(d time.Duration) float64 {
	return d.Seconds()
}

// LabeledCounter is a counter that supports label-based dimensions.
type LabeledCounter struct {
	mu       sync.Mutex
	counters map[string]*Counter
}

// NewLabeledCounter creates a new labeled counter.
func NewLabeledCounter() *LabeledCounter {
	return &LabeledCounter{
		counters: make(map[string]*Counter),
	}
}

// With returns a counter for the given labels, creating it if necessary.
func (lc *LabeledCounter) With(labels map[string]string) *Counter {
	key := SortLabels(labels)
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if c, ok := lc.counters[key]; ok {
		return c
	}
	c := newCounter(labels)
	lc.counters[key] = c
	return c
}

// LabeledHistogram is a histogram that supports label-based dimensions.
type LabeledHistogram struct {
	mu         sync.Mutex
	histograms map[string]*Histogram
}

// NewLabeledHistogram creates a new labeled histogram.
func NewLabeledHistogram() *LabeledHistogram {
	return &LabeledHistogram{
		histograms: make(map[string]*Histogram),
	}
}

// With returns a histogram for the given labels, creating it if necessary.
func (lh *LabeledHistogram) With(labels map[string]string) *Histogram {
	key := SortLabels(labels)
	lh.mu.Lock()
	defer lh.mu.Unlock()
	if h, ok := lh.histograms[key]; ok {
		return h
	}
	h := newHistogram(labels)
	lh.histograms[key] = h
	return h
}

// Quantile computes the q-th quantile from a sorted slice.
func Quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := q * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
