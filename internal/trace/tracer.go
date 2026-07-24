// Package trace provides distributed tracing for Promptsheon. It implements
// a lightweight span-based tracing system with a pluggable backend.
package trace

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// Status represents the outcome of a span.
type Status string

const (
	// StatusOK is a successful span status.
	StatusOK Status = "ok"
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

	// otelSpan is the OpenTelemetry span handle when this
	// Span was created by an OTelTracer. The field is
	// unexported because it carries a non-serialisable
	// runtime value; OTelTracer.Finish calls End() on it.
	otelSpan oteltrace.Span
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

// SpanFilter narrows the result set for ListSpans / GetTraceTree.
// OBS-TR-1: the read-side store keeps this filter contract so
// the API handler signature is unchanged after the SQLite
// writer was removed.
type SpanFilter struct {
	TraceID   string
	UserID    string
	Operation string
	Service   string
	Since     *time.Time
	Until     *time.Time
	Limit     int
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

	// Flush forces any buffered spans to the export pipeline. The
	// /ready handler calls this with a short timeout to verify the
	// OTel collector is accepting spans. Implementations that
	// don't buffer (the noop tracer) return nil.
	Flush(ctx context.Context) error
}

// Multi dispatches every Span to a list of wrapped Tracers. The
// Span returned to the caller is the primary tracer's span;
// each wrapped tracer also receives the same operation so they
// can independently export. OBS-2 uses Multi to forward HTTP
// spans to BOTH the SQLite tracer (for local /api/v1/traces
// browsing) AND the OTel tracer (for OTLP export). One tracer
// failure does not affect the others.
type Multi struct {
	primary Tracer
	others  []Tracer
}

// NewMulti returns a Multi that calls primary first, then every
// tracer in others. If primary is nil, NewMulti returns the first
// non-nil tracer in others; if all are nil, NewMulti returns a
// no-op tracer that drops spans.
func NewMulti(primary Tracer, others ...Tracer) Tracer {
	seen := false
	if primary != nil {
		seen = true
	}
	live := make([]Tracer, 0, len(others))
	for _, o := range others {
		if o != nil {
			live = append(live, o)
		}
	}
	if !seen {
		switch len(live) {
		case 0:
			return noopTracer{}
		case 1:
			return live[0]
		}
		return &Multi{primary: live[0], others: live[1:]}
	}
	return &Multi{primary: primary, others: live}
}

// NewNoopTracer returns a Tracer that drops every span. Used
// when OTel is not configured (OBS-TR-1).
func NewNoopTracer() Tracer { return noopTracer{} }

// Start creates a new root span on the primary and mirrors the
// operation across every wrapped tracer. The returned Span is
// owned by the primary; attribute setters propagate to the
// wrapped spans via the otelSpan field populated by OTelTracer.
func (m *Multi) Start(ctx context.Context, operation string) *Span {
	primary := m.primary.Start(ctx, operation)
	for _, t := range m.others {
		_ = t.Start(ctx, operation)
	}
	return primary
}

// StartChild creates a child span on the primary and every
// wrapped tracer.
func (m *Multi) StartChild(ctx context.Context, parent *Span, operation string) *Span {
	primary := m.primary.StartChild(ctx, parent, operation)
	for _, t := range m.others {
		_ = t.StartChild(ctx, parent, operation)
	}
	return primary
}

// Finish calls Finish on the primary and every wrapped tracer.
func (m *Multi) Finish(span *Span) error {
	var firstErr error
	if err := m.primary.Finish(span); err != nil {
		firstErr = err
	}
	for _, t := range m.others {
		if err := t.Finish(span); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Flush forwards to every wrapped tracer. The first error
// wins; the rest are still attempted. Used by the /ready
// handler to verify the OTel pipeline is accepting spans.
func (m *Multi) Flush(ctx context.Context) error {
	var firstErr error
	if err := m.primary.Flush(ctx); err != nil {
		firstErr = err
	}
	for _, t := range m.others {
		if err := t.Flush(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// noopTracer is the Tracer returned by NewMulti when every
// argument is nil. Useful in tests and when both SQLite and
// OTel are disabled.
type noopTracer struct{}

func (noopTracer) Start(_ context.Context, _ string) *Span {
	return &Span{ID: "noop", TraceID: "noop", StartedAt: time.Now()}
}
func (noopTracer) StartChild(_ context.Context, parent *Span, operation string) *Span {
	if parent == nil {
		return noopTracer{}.Start(context.Background(), operation)
	}
	return &Span{ID: "noop-child", TraceID: parent.TraceID, ParentID: parent.ID, StartedAt: time.Now()}
}
func (noopTracer) Finish(_ *Span) error          { return nil }
func (noopTracer) Flush(_ context.Context) error { return nil }

// Avoid unused-import lint when the package compiles without
// referencing sync below.
var _ = sync.Mutex{}

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
