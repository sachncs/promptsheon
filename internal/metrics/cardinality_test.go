package metrics

import (
	"fmt"
	"sync"
	"testing"
)

// OBS-8: the LabeledCounter / LabeledHistogram types were removed
// from the active Collector in commit f8644a7-ish. They live here
// as a historical reference and to exercise SortLabels + With
// for callers that might still want them in the future.

type LabeledCounter struct {
	mu       sync.Mutex
	counters map[string]*Counter
}

func NewLabeledCounter() *LabeledCounter {
	return &LabeledCounter{
		counters: make(map[string]*Counter),
	}
}

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

type LabeledHistogram struct {
	mu         sync.Mutex
	histograms map[string]*Histogram
}

func NewLabeledHistogram() *LabeledHistogram {
	return &LabeledHistogram{
		histograms: make(map[string]*Histogram),
	}
}

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

func TestLabeledCardinalityCountersDoNotExplode(t *testing.T) {
	t.Parallel()
	lc := NewLabeledCounter()
	for i := 0; i < 100; i++ {
		lc.With(map[string]string{"k": "v", "i": fmt.Sprintf("%d", i)})
	}
	if got := len(lc.counters); got != 100 {
		t.Errorf("counter cardinality: got %d, want 100", got)
	}
}
