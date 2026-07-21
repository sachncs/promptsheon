package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sachncs/promptsheon/internal/trace"
)

func (s *Server) handleListSpans(w http.ResponseWriter, r *http.Request) error {
	filter := trace.SpanFilter{
		Operation: r.URL.Query().Get("operation"),
		Service:   r.URL.Query().Get("service"),
		TraceID:   r.URL.Query().Get("trace_id"),
		Limit:     50,
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

	// Parse limit parameter
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return badRequest("invalid limit: must be an integer")
		}
		if n < 1 || n > 1000 {
			return badRequest("invalid limit: must be between 1 and 1000")
		}
		filter.Limit = n
	}

	spans, err := s.spanStore.ListSpans(r.Context(), &filter)
	if err != nil {
		return err
	}
	if spans == nil {
		spans = []*trace.Span{}
	}
	writeJSON(w, http.StatusOK, spans)
	return nil
}

func (s *Server) handleGetSpan(w http.ResponseWriter, r *http.Request) error {
	if s.spanStore == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "tracing not configured"}
	}
	id := r.PathValue("id")
	span, err := s.spanStore.GetSpan(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, span)
	return nil
}

func (s *Server) handleGetTraceTree(w http.ResponseWriter, r *http.Request) error {
	if s.spanStore == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "tracing not configured"}
	}
	traceID := r.PathValue("trace_id")
	spans, err := s.spanStore.GetTraceTree(r.Context(), traceID)
	if err != nil {
		return ErrNotFound
	}
	if len(spans) == 0 {
		return ErrNotFound
	}

	// Build tree structure
	type SpanNode struct {
		*trace.Span
		Children []*SpanNode `json:"children,omitempty"`
	}

	spanMap := make(map[string]*SpanNode)
	var roots []*SpanNode

	for _, sp := range spans {
		node := &SpanNode{Span: sp}
		spanMap[sp.ID] = node
	}

	for _, sp := range spans {
		node := spanMap[sp.ID]
		if sp.ParentID == "" {
			roots = append(roots, node)
		} else if parent, ok := spanMap[sp.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"trace_id": traceID,
		"spans":    spans,
		"tree":     roots,
		"count":    len(spans),
	})
	return nil
}
