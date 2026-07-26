package testutil

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// InMemoryCollector is an in-memory OTLP-compatible trace collector
// for tests. It wraps the OTel SDK's tracetest.InMemoryExporter
// so tests can assert on spans without starting a real OTLP server.
//
// Usage:
//
//	collector := testutil.NewInMemoryCollector()
//	defer collector.Shutdown(ctx)
//
//	// Create a tracer that writes to the collector
//	tracer := trace.NewTracerProvider(
//	    trace.WithSyncer(collector.Exporter()),
//    ).Tracer("test")
//
//	// ... do work that produces spans ...
//
//	// Assert on collected spans
//	spans := collector.Spans()
//	if len(spans) != 1 {
//	    t.Fatalf("expected 1 span, got %d", len(spans))
//	}
type InMemoryCollector struct {
	mu       sync.Mutex
	exporter *tracetest.InMemoryExporter
	provider *trace.TracerProvider
	spans    []tracetest.SpanStub
}

// NewInMemoryCollector creates a new in-memory collector and
// registers it as the global TracerProvider so any OTel tracer
// (including the one created by trace.NewOTelTracer) writes to it.
func NewInMemoryCollector() *InMemoryCollector {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Register as global provider so otel.Tracer() picks it up.
	otel.SetTracerProvider(provider)

	return &InMemoryCollector{
		exporter: exporter,
		provider: provider,
	}
}

// Exporter returns the underlying InMemoryExporter for use
// with a custom TracerProvider when the global registration
// is not desired.
func (c *InMemoryCollector) Exporter() *tracetest.InMemoryExporter {
	return c.exporter
}

// Provider returns the TracerProvider so callers can build
// tracers that explicitly target the collector.
func (c *InMemoryCollector) Provider() *trace.TracerProvider {
	return c.provider
}

// Spans returns all spans collected so far as SpanStubs.
// Each call returns a snapshot; concurrent calls are safe.
func (c *InMemoryCollector) Spans() []tracetest.SpanStub {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spans = c.exporter.GetSpans()
	out := make([]tracetest.SpanStub, len(c.spans))
	copy(out, c.spans)
	return out
}

// Reset clears all collected spans. Useful between sub-tests.
func (c *InMemoryCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exporter.Reset()
	c.spans = nil
}

// Shutdown flushes any buffered spans and shuts down the provider.
func (c *InMemoryCollector) Shutdown(ctx context.Context) error {
	return c.provider.Shutdown(ctx)
}
