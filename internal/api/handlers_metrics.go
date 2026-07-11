package api

import (
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
type UsageTracker struct {
	mu              sync.RWMutex
	capabilityUsage map[string]*UsageCount
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

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		capabilityUsage: make(map[string]*UsageCount),
	}
}

// RecordCapabilityUsage records a Capability usage event.
func (t *UsageTracker) RecordCapabilityUsage(id, name string, tokens int, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.capabilityUsage[id]
	if !ok {
		usage = &UsageCount{ID: id, Name: name}
		t.capabilityUsage[id] = usage
	}

	usage.Count++
	usage.LastUsed = time.Now()
	usage.AvgTokens = (usage.AvgTokens*float64(usage.Count-1) + float64(tokens)) / float64(usage.Count)
	usage.AvgLatency = (usage.AvgLatency*float64(usage.Count-1) + latencyMs) / float64(usage.Count)
}

// GetTopCapabilities returns the most-used Capabilities.
func (t *UsageTracker) GetTopCapabilities(limit int) []*UsageCount {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var usages []*UsageCount
	for _, u := range t.capabilityUsage {
		usages = append(usages, u)
	}

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
