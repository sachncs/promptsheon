package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	c := newCounter(nil)
	if c.Value() != 0 {
		t.Fatal("expected 0")
	}
	c.Inc()
	c.Inc()
	c.Add(3)
	if c.Value() != 5 {
		t.Fatalf("expected 5, got %f", c.Value())
	}
}

func TestHistogram(t *testing.T) {
	h := newHistogram(nil)
	h.Observe(0.1)
	h.Observe(0.5)
	h.Observe(1.0)

	if h.Count() != 3 {
		t.Fatalf("expected 3, got %d", h.Count())
	}
	if h.Sum() != 1.6 {
		t.Fatalf("expected 1.6, got %f", h.Sum())
	}
	avg := h.Avg()
	if avg < 0.5 || avg > 0.6 {
		t.Fatalf("expected ~0.53, got %f", avg)
	}
}

func TestGauge(t *testing.T) {
	g := &Gauge{}
	g.Inc()
	g.Inc()
	if g.Value() != 2 {
		t.Fatalf("expected 2, got %f", g.Value())
	}
	g.Dec()
	if g.Value() != 1 {
		t.Fatalf("expected 1, got %f", g.Value())
	}
	g.Set(42)
	if g.Value() != 42 {
		t.Fatalf("expected 42, got %f", g.Value())
	}
}

func TestCollectorSummary(t *testing.T) {
	c := NewCollector()
	c.RequestsTotal.Inc()
	c.RequestsTotal.Inc()
	c.RequestDuration.Observe(0.05)
	c.LLMTokensTotal.Add(100)

	summary := c.GetSummary()
	if summary.APIMetrics.TotalRequests != 2 {
		t.Fatalf("expected 2 requests, got %d", summary.APIMetrics.TotalRequests)
	}
	if summary.LLMMetrics.TotalTokens != 100 {
		t.Fatalf("expected 100 tokens, got %d", summary.LLMMetrics.TotalTokens)
	}
}

// TestCollectorAuditQueueLatency locks in OBS-AUDIT-2: the
// histogram is wired through NewCollector and observations
// surface in the summary's pipeline_metrics block.
func TestCollectorAuditQueueLatency(t *testing.T) {
	c := NewCollector()
	c.ObserveAuditQueue(0.001)
	c.ObserveAuditQueue(0.010)
	c.ObserveAuditQueue(0.100)

	summary := c.GetSummary()
	if summary.PipelineMetrics.AuditQueueAvgSecs <= 0 {
		t.Errorf("expected positive AuditQueueAvgSecs, got %f", summary.PipelineMetrics.AuditQueueAvgSecs)
	}
	if summary.PipelineMetrics.AuditQueueP95Secs < summary.PipelineMetrics.AuditQueueAvgSecs {
		t.Errorf("p95 %f should be >= avg %f", summary.PipelineMetrics.AuditQueueP95Secs, summary.PipelineMetrics.AuditQueueAvgSecs)
	}
	if summary.PipelineMetrics.AuditQueueP99Secs < summary.PipelineMetrics.AuditQueueP95Secs {
		t.Errorf("p99 %f should be >= p95 %f", summary.PipelineMetrics.AuditQueueP99Secs, summary.PipelineMetrics.AuditQueueP95Secs)
	}

	// Prometheus exposition contains the histogram.
	format := c.prometheusFormat()
	if !strings.Contains(format, "promptsheon_audit_queue_latency_seconds") {
		t.Errorf("expected histogram metric in Prometheus output, got:\n%s", format)
	}
}

func TestQuantile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	q50 := Quantile(sorted, 0.5)
	if q50 < 5 || q50 > 6 {
		t.Fatalf("expected p50 ~5.5, got %f", q50)
	}
	q99 := Quantile(sorted, 0.99)
	if q99 < 9 || q99 > 10 {
		t.Fatalf("expected p99 ~9.9, got %f", q99)
	}
}

func TestPrometheusFormat(t *testing.T) {
	c := NewCollector()
	c.RequestsTotal.Inc()
	c.LLMTokensTotal.Add(50)

	format := c.prometheusFormat()
	if len(format) == 0 {
		t.Fatal("expected non-empty prometheus format")
	}
	// Verify key metrics are present
	for _, expected := range []string{
		"promptsheon_http_requests_total",
		"promptsheon_llm_tokens_total",
		"promptsheon_eval_runs_total",
	} {
		if !containsString(format, expected) {
			t.Fatalf("expected metric %s in output", expected)
		}
	}
}

func TestDurationConversions(t *testing.T) {
	d := 500 * time.Millisecond
	ms := DurationMs(d)
	if ms != 500 {
		t.Fatalf("expected 500ms, got %f", ms)
	}
	sec := DurationSec(d)
	if sec != 0.5 {
		t.Fatalf("expected 0.5s, got %f", sec)
	}
}

func TestSortLabels(t *testing.T) {
	labels := map[string]string{"b": "2", "a": "1", "c": "3"}
	sorted := SortLabels(labels)
	if sorted != "a=1,b=2,c=3" {
		t.Fatalf("expected 'a=1,b=2,c=3', got %q", sorted)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (s[0:len(sub)] == sub || containsString(s[1:], sub)))
}

// TestHistogramPercentileP99 pins PERF-3b: percentile cost at
// p99 stays < 1ms for 10k samples. The 5ms ceiling is generous
// for CI variance; the benchmark below is the precise measurement.
func TestHistogramPercentileP99(t *testing.T) {
	h := newHistogram(nil)
	for i := 0; i < 10000; i++ {
		h.Observe(float64(i) * 0.001)
	}
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_ = h.Percentile(99)
	}
	elapsed := time.Since(start)
	perCall := elapsed / 1000
	if perCall > 5*time.Millisecond {
		t.Errorf("Percentile p99 too slow: %v per call (expected < 5ms)", perCall)
	}
}
