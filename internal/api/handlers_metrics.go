package api

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

// UsageTracker tracks usage counts for prompts and agents.
type UsageTracker struct {
	mu          sync.RWMutex
	promptUsage map[string]*UsageCount
	agentUsage  map[string]*UsageCount
}

// UsageCount tracks usage statistics for a resource.
type UsageCount struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Count     int       `json:"count"`
	LastUsed  time.Time `json:"last_used"`
	AvgTokens float64   `json:"avg_tokens,omitempty"`
	AvgLatency float64  `json:"avg_latency_ms,omitempty"`
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		promptUsage: make(map[string]*UsageCount),
		agentUsage:  make(map[string]*UsageCount),
	}
}

// RecordPromptUsage records a prompt usage event.
func (t *UsageTracker) RecordPromptUsage(id, name string, tokens int, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.promptUsage[id]
	if !ok {
		usage = &UsageCount{ID: id, Name: name}
		t.promptUsage[id] = usage
	}

	usage.Count++
	usage.LastUsed = time.Now()
	usage.AvgTokens = (usage.AvgTokens*float64(usage.Count-1) + float64(tokens)) / float64(usage.Count)
	usage.AvgLatency = (usage.AvgLatency*float64(usage.Count-1) + latencyMs) / float64(usage.Count)
}

// RecordAgentUsage records an agent usage event.
func (t *UsageTracker) RecordAgentUsage(id, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.agentUsage[id]
	if !ok {
		usage = &UsageCount{ID: id, Name: name}
		t.agentUsage[id] = usage
	}

	usage.Count++
	usage.LastUsed = time.Now()
}

// GetTopPrompts returns the most used prompts.
func (t *UsageTracker) GetTopPrompts(limit int) []*UsageCount {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var usages []*UsageCount
	for _, u := range t.promptUsage {
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

// GetTopAgents returns the most used agents.
func (t *UsageTracker) GetTopAgents(limit int) []*UsageCount {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var usages []*UsageCount
	for _, u := range t.agentUsage {
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

func (s *Server) handleMetricsSummary(w http.ResponseWriter, r *http.Request) error {
	summary := s.collector.GetSummary()
	writeJSON(w, http.StatusOK, summary)
	return nil
}

func (s *Server) handleTopPrompts(w http.ResponseWriter, r *http.Request) error {
	if s.usageTracker == nil {
		writeJSON(w, http.StatusOK, []*UsageCount{})
		return nil
	}

	limit := 10
	usages := s.usageTracker.GetTopPrompts(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"prompts": usages,
		"total":   len(usages),
	})
	return nil
}

func (s *Server) handleTopAgents(w http.ResponseWriter, r *http.Request) error {
	if s.usageTracker == nil {
		writeJSON(w, http.StatusOK, []*UsageCount{})
		return nil
	}

	limit := 10
	usages := s.usageTracker.GetTopAgents(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": usages,
		"total":  len(usages),
	})
	return nil
}
