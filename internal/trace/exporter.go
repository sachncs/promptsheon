package trace

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InitTracerProvider initializes the OpenTelemetry TracerProvider with an OTLP exporter.
// If endpoint is empty, it uses a no-op exporter for development.
// Returns the TracerProvider for shutdown and any error encountered.
func InitTracerProvider(serviceName, endpoint string, insecureConn bool) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Create resource with service info
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	// Create exporter
	var (
		exporter sdktrace.SpanExporter
		err      error
	)
	if endpoint != "" {
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}
		if insecureConn {
			opts = append(opts, otlptracegrpc.WithDialOption(
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			))
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", err)
		}
	} else {
		// No endpoint configured - use noop exporter
		exporter = newnoopExporter()
	}

	// Sample ratio. OBS-TR-3: PROMPTSHEON_OTEL_SAMPLE_RATIO
	// overrides the hard-coded 0.05 (5%). Default 1.0 means
	// every span ships, which is the previous behaviour before the
	// 5% cap was added.
	sampleRatio := 1.0
	if v := os.Getenv("PROMPTSHEON_OTEL_SAMPLE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			sampleRatio = f
		}
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// Default to ParentBased(TraceIDRatioBased(0.05)) so a
		// single noisy deployment does not ship every span to the
		// collector. PROMPTSHEON_OTEL_SAMPLE_RATIO overrides this.
		// The previous default of AlwaysSample() meant production
		// traffic generated 100% of spans, which is the single
		// biggest cost on most OTel deployments.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
	)

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	// Set global propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// noopExporter is a no-op span exporter for development.
type noopExporter struct{}

func newnoopExporter() *noopExporter {
	return &noopExporter{}
}

func (e *noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noopExporter) Shutdown(_ context.Context) error {
	return nil
}
