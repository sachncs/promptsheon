// Package trace provides distributed tracing for Promptsheon. It implements
// a lightweight span-based tracing system with a pluggable backend.
package trace

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// otelSpanKey is the context key for storing OTel spans.
type otelSpanKey struct{}

// OTelTracer wraps the OpenTelemetry tracer and implements the Tracer interface.
type OTelTracer struct {
	tracer      oteltrace.Tracer
	serviceName string
	// provider is the SDK TracerProvider; Flush uses it to
	// force the export pipeline to drain. nil when the
	// Tracer was constructed without an SDK (rare in
	// production; tests use the noop tracer).
	provider any // sdktrace.TracerProvider; type-erased to avoid the SDK import
}

// Flush forces any buffered spans to the export pipeline.
// Implemented via the SDK TracerProvider.ForceFlush; returns
// nil when the provider isn't an SDK provider (which is the
// noop case). The interface is type-asserted to keep the
// import surface small.
func (t *OTelTracer) Flush(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	if p, ok := t.provider.(interface{ ForceFlush(context.Context) error }); ok {
		return p.ForceFlush(ctx)
	}
	return nil
}

// NewOTelTracer creates a new OpenTelemetry-backed tracer that
// uses the global OTel TracerProvider. The Flush call is a
// no-op when the global provider is the noop (test/dev).
func NewOTelTracer(serviceName string) *OTelTracer {
	tracer := otel.Tracer(serviceName)
	return &OTelTracer{
		tracer:      tracer,
		serviceName: serviceName,
		provider:    otel.GetTracerProvider(),
	}
}

// Start creates a new root span using OpenTelemetry.
func (t *OTelTracer) Start(ctx context.Context, operation string) *Span {
	ctx, otelSpan := t.tracer.Start(ctx, operation,
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
	)

	span := &Span{
		ID:        otelSpan.SpanContext().SpanID().String(),
		TraceID:   otelSpan.SpanContext().TraceID().String(),
		Operation: operation,
		Service:   t.serviceName,
		Status:    StatusUnset,
		StartedAt: time.Now(),
		otelSpan:  otelSpan,
	}

	return span
}

// StartChild creates a span parented to the given parent.
func (t *OTelTracer) StartChild(ctx context.Context, parent *Span, operation string) *Span {
	if parent == nil {
		return t.Start(ctx, operation)
	}

	// Try to retrieve the OTel span from context for proper parent-child linking
	var opts []oteltrace.SpanStartOption
	if parentSpan, ok := ctx.Value(otelSpanKey{}).(oteltrace.Span); ok {
		opts = append(opts, oteltrace.WithLinks(oteltrace.Link{
			SpanContext: parentSpan.SpanContext(),
		}))
	}

	ctx, otelSpan := t.tracer.Start(ctx, operation, opts...)

	span := &Span{
		ID:        otelSpan.SpanContext().SpanID().String(),
		TraceID:   otelSpan.SpanContext().TraceID().String(),
		ParentID:  parent.ID,
		Operation: operation,
		Service:   t.serviceName,
		Status:    StatusUnset,
		StartedAt: time.Now(),
		otelSpan:  otelSpan,
	}

	// Store OTel span in context for child span linking so
	// downstream callers see the in-flight span via
	// SpanFromContext. The new context is not propagated back
	// to the caller because the Span value is the public handle.
	ctx = context.WithValue(ctx, otelSpanKey{}, otelSpan)

	return span
}

// Finish records a completed span in OpenTelemetry. This is the
// fix for the OBS-3 issue: the previous implementation never
// called otelSpan.End(), so the OTel exporter saw no span at
// all even when the daemon was configured with an OTLP endpoint.
func (t *OTelTracer) Finish(span *Span) error {
	if span == nil {
		return nil
	}

	span.Finish()
	if span.otelSpan != nil {
		for k, v := range span.Attributes {
			span.otelSpan.SetAttributes(attribute.String(k, v))
		}
		if span.Error != "" {
			span.otelSpan.SetStatus(otelcodes.Error, span.Error)
			span.otelSpan.RecordError(fmt.Errorf("%s", span.Error))
		} else {
			span.otelSpan.SetStatus(otelcodes.Ok, "")
		}
		span.otelSpan.End()
	}
	return nil
}

// FinishWithError records a completed span with error information.
func (t *OTelTracer) FinishWithError(span *Span, err error) error {
	if span == nil {
		return nil
	}

	if err != nil {
		span.SetError(err)
	}

	return t.Finish(span)
}

// RecordSpan records a span to the OTel exporter. This is a helper method
// for cases where you have direct access to the OTel span.
func (t *OTelTracer) RecordSpan(otelSpan oteltrace.Span, span *Span) {
	if otelSpan == nil || span == nil {
		return
	}

	// Set attributes
	for k, v := range span.Attributes {
		otelSpan.SetAttributes(otelAttributeString(k, v))
	}

	// Set error if present
	if span.Error != "" {
		otelSpan.SetStatus(otelcodes.Error, span.Error)
		otelSpan.RecordError(fmt.Errorf("%s", span.Error))
	} else {
		otelSpan.SetStatus(otelcodes.Ok, "")
	}

	otelSpan.End()
}

// otelAttributeString creates a string attribute for OpenTelemetry.
func otelAttributeString(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}
