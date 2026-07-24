package metrics

import (
	"testing"
	"time"
)

func BenchmarkCounterInc(b *testing.B) {
	c := newCounter(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkCounterAdd(b *testing.B) {
	c := newCounter(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Add(1.5)
	}
}

func BenchmarkHistogramObserve(b *testing.B) {
	h := newHistogram(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(float64(i%100) / 100.0)
	}
}

// BenchmarkHistogramPercentile pins PERF-3b: Percentile on a
// 10k-sample histogram stays under 1ms p99. The implementation
// walks the fixed-bucket counts (O(buckets)), not the raw values.
func BenchmarkHistogramPercentile(b *testing.B) {
	h := newHistogram(nil)
	for i := 0; i < 10000; i++ {
		h.Observe(float64(i) * 0.001)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Percentile(99)
	}
}

func BenchmarkLabeledCounter(b *testing.B) {
	lc := NewLabeledCounter()
	labels := map[string]string{"method": "GET", "status": "200"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lc.With(labels).Inc()
	}
}

func BenchmarkLabeledHistogram(b *testing.B) {
	lh := NewLabeledHistogram()
	labels := map[string]string{"method": "GET", "status": "200"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lh.With(labels).Observe(float64(i%100) / 100.0)
	}
}

func BenchmarkSortLabels(b *testing.B) {
	labels := map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/api/v1/prompts",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SortLabels(labels)
	}
}

func BenchmarkDurationSec(b *testing.B) {
	d := 5 * time.Second
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DurationSec(d)
	}
}

func BenchmarkGetSummary(b *testing.B) {
	c := NewCollector()
	// Add some data
	for i := 0; i < 100; i++ {
		c.RequestsTotal.Inc()
		c.RequestDuration.Observe(float64(i) / 100.0)
		if i%10 == 0 {
			c.ErrorsTotal.Inc()
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.GetSummary()
	}
}
