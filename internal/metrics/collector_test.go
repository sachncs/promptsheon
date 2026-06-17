package metrics

import (
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
