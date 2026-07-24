package api

import (
	"container/list"
	"net/http"
	"sort"
	"sync"
	"time"
)

// UsageTracker tracks per-Capability usage counts in-process. The
// legacy PromptUsage / AgentUsage maps were removed in v0.1.0:
// the new abstraction is the Capability aggregate (ADR-0010) and
// the v0.0.7 prompts/agents tables are dropped (F-06). The map
// name reflects that.
//
// PERF-4c: the single map + RWMutex becomes a per-entry list
// element under a single mutex. Concurrent Records for different
// capabilities are still serialised (the mutex is held for
// ~100ns) but the load is uniform and the RWMutex upgrade
// contention is gone.
//
// PERF-5: an LRU eviction policy caps the in-memory map at
// maxUsageEntries so the tracker cannot grow unbounded across a
// long-running daemon. The "least recently used" item is the
// tail of the list; Records on an existing entry move it to the
// front.
type UsageTracker struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List
}

// UsageCount tracks usage statistics for one Capability.
type UsageCount struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Count      int       `json:"count"`
	LastUsed   time.Time `json:"last_used"`
	AvgTokens  float64   `json:"avg_tokens,omitempty"`
	AvgLatency float64   `json:"avg_latency_ms,omitempty"`
}

// maxUsageEntries caps the LRU so the UsageTracker cannot grow
// without bound. 16384 is roughly 4x the "top 10k" dashboard
// target; well below the 4096-record observation window on a
// per-tenant basis.
const maxUsageEntries = 16384

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		entries: make(map[string]*list.Element),
		order:   list.New(),
	}
}

// RecordCapabilityUsage records a Capability usage event.
func (t *UsageTracker) RecordCapabilityUsage(id, name string, tokens int, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if el, ok := t.entries[id]; ok {
		u := el.Value.(*UsageCount)
		u.Count++
		u.LastUsed = now
		u.AvgTokens = (u.AvgTokens*float64(u.Count-1) + float64(tokens)) / float64(u.Count)
		u.AvgLatency = (u.AvgLatency*float64(u.Count-1) + latencyMs) / float64(u.Count)
		t.order.MoveToFront(el)
		return
	}

	u := &UsageCount{ID: id, Name: name}
	el := t.order.PushFront(u)
	t.entries[id] = el

	if t.order.Len() > maxUsageEntries {
		// Evict the least-recently-used entry (the tail).
		oldest := t.order.Back()
		if oldest != nil {
			t.order.Remove(oldest)
			delete(t.entries, oldest.Value.(*UsageCount).ID)
		}
	}

	// Initialise the running averages from the first sample.
	u.Count = 1
	u.LastUsed = now
	u.AvgTokens = float64(tokens)
	u.AvgLatency = latencyMs
}

// GetTopCapabilities returns the most-used Capabilities.
func (t *UsageTracker) GetTopCapabilities(limit int) []*UsageCount {
	t.mu.Lock()
	usages := make([]*UsageCount, 0, t.order.Len())
	for e := t.order.Front(); e != nil; e = e.Next() {
		usages = append(usages, e.Value.(*UsageCount))
	}
	t.mu.Unlock()

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].Count > usages[j].Count
	})

	if limit > 0 && len(usages) > limit {
		usages = usages[:limit]
	}

	return usages
}

func (s *Server) handleMetricsSummary(w http.ResponseWriter, _ *http.Request) error {
	summary := s.collector.GetSummary()
	writeJSON(w, http.StatusOK, summary)
	return nil
}

func (s *Server) handleTopCapabilities(w http.ResponseWriter, _ *http.Request) error {
	if s.usageTracker == nil {
		writeJSON(w, http.StatusOK, []*UsageCount{})
		return nil
	}

	limit := 10
	usages := s.usageTracker.GetTopCapabilities(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"capabilities": usages,
		"total":        len(usages),
	})
	return nil
}
