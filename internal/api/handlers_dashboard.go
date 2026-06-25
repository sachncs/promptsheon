package api

import (
	"net/http"
	"time"

	"github.com/sachn-cs/promptsheon/internal/trace"
)

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) error {
	summary := s.collector.GetSummary()

	// Add trace stats
	if s.spans != nil {
		spans, _ := s.spans.ListSpans(r.Context(), trace.SpanFilter{Limit: 1000})
		summary.TraceStats = &TraceStats{
			TotalSpans:      len(spans),
			RecentTraces:    getRecentTraceCount(spans, 1*time.Hour),
			AvgSpanDuration: getAvgSpanDuration(spans),
			ErrorSpans:      getErrorSpanCount(spans),
		}
	}

	writeJSON(w, http.StatusOK, summary)
	return nil
}

type TraceStats struct {
	TotalSpans      int     `json:"total_spans"`
	RecentTraces    int     `json:"recent_traces_1h"`
	AvgSpanDuration float64 `json:"avg_span_duration_ms"`
	ErrorSpans      int     `json:"error_spans"`
}

func getRecentTraceCount(spans []*trace.Span, within time.Duration) int {
	cutoff := time.Now().Add(-within)
	count := 0
	for _, s := range spans {
		if s.StartedAt.After(cutoff) {
			count++
		}
	}
	return count
}

func getAvgSpanDuration(spans []*trace.Span) float64 {
	if len(spans) == 0 {
		return 0
	}
	total := int64(0)
	for _, s := range spans {
		total += s.DurationMs
	}
	return float64(total) / float64(len(spans))
}

func getErrorSpanCount(spans []*trace.Span) int {
	count := 0
	for _, s := range spans {
		if s.Status == trace.StatusError {
			count++
		}
	}
	return count
}

func (s *Server) handleSearchSpans(w http.ResponseWriter, r *http.Request) error {
	filter := trace.SpanFilter{
		Operation: r.URL.Query().Get("operation"),
		Service:   r.URL.Query().Get("service"),
		TraceID:   r.URL.Query().Get("trace_id"),
		Limit:     100,
	}

	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := r.URL.Query().Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = &t
		}
	}

	spans, err := s.spans.ListSpans(r.Context(), filter)
	if err != nil {
		return err
	}
	if spans == nil {
		spans = []*trace.Span{}
	}

	// Group by trace_id
	traceGroups := make(map[string][]*trace.Span)
	for _, sp := range spans {
		traceGroups[sp.TraceID] = append(traceGroups[sp.TraceID], sp)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"spans":       spans,
		"trace_count": len(traceGroups),
		"traces":      traceGroups,
	})
	return nil
}
