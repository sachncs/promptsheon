package trace

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config is the resolved OTel configuration. The settings
// layer merges env (PROMPTSHEON_OTEL_*) + DB rows into this
// struct on every call to NewTracerProvider / Reconfigure.
// Fields that read from env directly inside this package (the
// sample ratio) are an implementation detail of the boot path
// — operators who want runtime tunability use the settings
// layer; the env-read path stays for the production-default
// path.
type Config struct {
	ServiceName string
	Endpoint    string
	Insecure    bool
	SampleRatio float64
}

// FromEnv returns a Config populated from PROMPTSHEON_OTEL_*
// env-var defaults. The settings layer overrides individual
// fields before calling NewTracerProvider.
func FromEnv(serviceName string) Config {
	cfg := Config{ServiceName: serviceName, SampleRatio: 1.0}
	if v := getEnv("PROMPTSHEON_OTEL_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	cfg.Insecure = getEnvBool("PROMPTSHEON_OTEL_INSECURE")
	if v := getEnv("PROMPTSHEON_OTEL_SAMPLE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			cfg.SampleRatio = f
		}
	}
	return cfg
}

// TracerProvider is a thin wrapper over the SDK TracerProvider
// that exposes a Reconfigure method for hot reload. The
// settings layer (commit A3) calls Reconfigure when otl.* keys
// change; Reconfigure flushes pending spans on the OLD provider
// before swapping in the new one so no span is dropped.
type TracerProvider struct {
	mu       sync.Mutex
	provider *sdktrace.TracerProvider
}

// NewTracerProvider constructs the SDK TracerProvider from
// cfg, sets it as the global, and returns the wrapper. If
// cfg.Endpoint is empty, a no-op exporter is used (dev mode).
func NewTracerProvider(ctx context.Context, cfg Config) (*TracerProvider, error) {
	tp, err := newSDKProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}
	w := &TracerProvider{provider: tp}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return w, nil
}

// Reconfigure swaps the underlying TracerProvider in place.
// Flushes pending spans on the OLD provider before the swap
// so no span is dropped.
func (t *TracerProvider) Reconfigure(ctx context.Context, cfg Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	old := t.provider
	newTP, err := newSDKProvider(ctx, cfg)
	if err != nil {
		return err
	}
	// Flush the OLD provider's pending spans before swap.
	if old != nil {
		_ = old.ForceFlush(ctx)
	}
	t.provider = newTP
	otel.SetTracerProvider(newTP)
	if old != nil {
		_ = old.Shutdown(ctx)
	}
	return nil
}

// Shutdown flushes + closes the current TracerProvider.
func (t *TracerProvider) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// newSDKProvider builds a fresh SDK TracerProvider from cfg.
// Private — the public surface is TracerProvider.New /
// Reconfigure / Shutdown so callers can't reach past the
// settings-aware wrapper.
func newSDKProvider(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
	)
	var (
		exporter sdktrace.SpanExporter
		err      error
	)
	if cfg.Endpoint != "" {
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithDialOption(
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			))
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", err)
		}
	} else {
		exporter = newnoopExporter()
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	), nil
}

// InitTracerProvider preserves the production boot-path
// shape: env-var defaults + the SDK side effects. Calls
// NewTracerProvider internally so the global TracerProvider
// is set. cmd/promptsheond/main.go is the only caller.
func InitTracerProvider(serviceName, endpoint string, insecureConn bool) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	cfg := Config{
		ServiceName: serviceName,
		Endpoint:    endpoint,
		Insecure:    insecureConn,
		SampleRatio: 1.0,
	}
	if v := getEnv("PROMPTSHEON_OTEL_SAMPLE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			cfg.SampleRatio = f
		}
	}
	w, err := NewTracerProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return w.provider, nil
}

// noopExporter is a no-op span exporter for development.
type noopExporter struct{}

func newnoopExporter() *noopExporter { return &noopExporter{} }

func (e *noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noopExporter) Shutdown(_ context.Context) error { return nil }

// getenv is a thin wrapper around os.Getenv that we unit-test
// with a custom EnvSource in commit A3. The default uses the
// real OS env (the production boot path).
func getEnv(key string) string { return osGetenv(key) }

// getEnvBool is the boolean sibling of getEnv. Returns false
// on parse failure (matches the "no env = no behavior" rule).
func getEnvBool(key string) bool { return osGetenv(key) == "true" }

// osGetenv is the indirection point for tests to swap in a
// static env map. The default uses the real OS env.
var osGetenv = os.Getenv
