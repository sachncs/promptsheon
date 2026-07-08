// Package trace provides distributed tracing for Promptsheon. It implements
// a lightweight span-based tracing system with a pluggable backend.
package trace

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Status represents the outcome of a span.
type Status string

const (
	// StatusOK is a successful span status.
	StatusOK    Status = "ok"
	// StatusError is an error span status.
	StatusError Status = "error"
	// StatusUnset is the default unset span status.
	StatusUnset Status = ""
)

// Span represents a single unit of work within a trace.
type Span struct {
	ID         string            `json:"id"`
	TraceID    string            `json:"trace_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Operation  string            `json:"operation"`
	Service    string            `json:"service"`
	Status     Status            `json:"status"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Error      string            `json:"error,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    *time.Time        `json:"ended_at,omitempty"`
	DurationMs int64             `json:"duration_ms"`
}

// Finish completes a span and records the end time.
func (s *Span) Finish() {
	now := time.Now()
	s.EndedAt = &now
	s.DurationMs = now.Sub(s.StartedAt).Milliseconds()
}

// SetError records an error on the span.
func (s *Span) SetError(err error) {
	s.Status = StatusError
	if err != nil {
		s.Error = err.Error()
	}
}

// SetAttribute adds a key-value attribute to the span.
func (s *Span) SetAttribute(key, value string) {
	if s.Attributes == nil {
		s.Attributes = make(map[string]string)
	}
	s.Attributes[key] = value
}

// Tracer creates and manages spans.
type Tracer interface {
	// Start creates a new root span.
	Start(ctx context.Context, operation string) *Span

	// StartChild creates a span parented to the given parent.
	StartChild(ctx context.Context, parent *Span, operation string) *Span

	// Finish records a completed span.
	Finish(span *Span) error
}

// contextKey is the unexported type for context keys in this package.
type contextKey string

const (
	spanContextKey contextKey = "span"
	// TraceIDContextKey is the context key for trace IDs.
	TraceIDContextKey contextKey = "trace_id"
	// RequestIDContextKey is the context key for request IDs.
	RequestIDContextKey contextKey = "request_id"
	// UserIDContextKey is the context key for user IDs.
	UserIDContextKey contextKey = "user_id"
)

// IDFromContext returns the trace ID from the context, if any.
func IDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(TraceIDContextKey).(string)
	return id, ok
}

// RequestIDFromContext returns the request ID from the context, if any.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(RequestIDContextKey).(string)
	return id, ok
}

// UserIDFromContext returns the user ID from the context, if any.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	return id, ok
}

// WithTraceID returns a new context with the trace ID attached.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, id)
}

// WithRequestID returns a new context with the request ID attached.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, id)
}

// WithUserID returns a new context with the user ID attached.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDContextKey, id)
}

// SpanFromContext returns the current span from the context, if any.
func SpanFromContext(ctx context.Context) (*Span, bool) {
	s, ok := ctx.Value(spanContextKey).(*Span)
	return s, ok
}

// WithSpanContext returns a new context with the span attached.
func WithSpanContext(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, spanContextKey, s)
}

// idCounter is an atomic counter that disambiguates IDs created in
// the same nanosecond. UnixNano alone is not unique when two spans
// are created back-to-back on a fast machine, so we append a
// process-wide counter to guarantee uniqueness.
var idCounter atomic.Uint64

// GenerateID creates a unique span ID. The ID is collision-safe even
// for two spans created in the same nanosecond because we mix the
// UnixNano timestamp with an atomic counter.
func GenerateID() string {
	n := idCounter.Add(1)
	return fmt.Sprintf("span-%d-%d", time.Now().UnixNano(), n)
}

// GenerateTraceID creates a unique trace identifier. As with
// GenerateID, the timestamp is combined with a counter to guarantee
// uniqueness under burst creation.
func GenerateTraceID() string {
	n := idCounter.Add(1)
	return fmt.Sprintf("trace-%d-%d", time.Now().UnixNano(), n)
}
