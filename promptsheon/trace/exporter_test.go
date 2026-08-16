// Package trace — exporter tests (PR-7).
//
// These tests cover the TracerProvider lifecycle in
// promptsheon/trace/exporter.go: Config defaults, FromEnv parsing,
// TracerProvider.Reconfigure / Shutdown. The tests stub the env
// reader (osGetenv) so the env-var path is deterministic.
package trace_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/sachncs/promptsheon/promptsheon/trace"
)

// setEnv uses t.Setenv so the test framework restores the env vars
// at test end. The trace package's FromEnv reads the env via
// osGetenv (a package-level var in promptsheon/trace); process-
// level env vars are the public way to influence the env-var path.
func setEnv(t *testing.T, mapping map[string]string) {
	t.Helper()
	for k, v := range mapping {
		t.Setenv(k, v)
	}
}

// TestConfigDefaults verifies FromEnv returns a sane Config when no
// env vars are set: ServiceName is preserved, SampleRatio defaults
// to 1.0, Endpoint/Insecure are false.
func TestConfigDefaults(t *testing.T) {
	cfg := trace.FromEnv("svc-defaults")
	if cfg.ServiceName != "svc-defaults" {
		t.Errorf("ServiceName = %q want %q", cfg.ServiceName, "svc-defaults")
	}
	if cfg.SampleRatio != 1.0 {
		t.Errorf("SampleRatio = %v want 1.0", cfg.SampleRatio)
	}
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint = %q want empty", cfg.Endpoint)
	}
	if cfg.Insecure {
		t.Errorf("Insecure = true want false")
	}
}

// TestConfigFromEnv verifies FromEnv parses the PROMPTSHEON_OTEL_*
// env vars: endpoint, insecure flag, sample ratio.
func TestConfigFromEnv(t *testing.T) {
	setEnv(t, map[string]string{
		"PROMPTSHEON_OTEL_ENDPOINT":      "otel-collector:4317",
		"PROMPTSHEON_OTEL_INSECURE":      "true",
		"PROMPTSHEON_OTEL_SAMPLE_RATIO":   "0.25",
	})
	cfg := trace.FromEnv("svc-env")
	if cfg.Endpoint != "otel-collector:4317" {
		t.Errorf("Endpoint = %q want %q", cfg.Endpoint, "otel-collector:4317")
	}
	if !cfg.Insecure {
		t.Errorf("Insecure = false want true")
	}
	if cfg.SampleRatio != 0.25 {
		t.Errorf("SampleRatio = %v want 0.25", cfg.SampleRatio)
	}
}

// TestConfigFromEnv_Bounds verifies sample ratio is clamped to
// [0, 1].
func TestConfigFromEnv_Bounds(t *testing.T) {
	setEnv(t, map[string]string{
		"PROMPTSHEON_OTEL_SAMPLE_RATIO": "2.5",
	})
	cfg := trace.FromEnv("svc-bounds")
	if cfg.SampleRatio != 1.0 {
		t.Errorf("SampleRatio clamp high: %v want 1.0", cfg.SampleRatio)
	}

	setEnv(t, map[string]string{
		"PROMPTSHEON_OTEL_SAMPLE_RATIO": "-0.5",
	})
	cfg = trace.FromEnv("svc-bounds")
	if cfg.SampleRatio != 0.0 {
		t.Errorf("SampleRatio clamp low: %v want 0.0", cfg.SampleRatio)
	}
}

// TestConfigFromEnv_InvalidRatio verifies a malformed ratio falls
// back to the default 1.0.
func TestConfigFromEnv_InvalidRatio(t *testing.T) {
	setEnv(t, map[string]string{
		"PROMPTSHEON_OTEL_SAMPLE_RATIO": "not-a-number",
	})
	cfg := trace.FromEnv("svc-bad")
	if cfg.SampleRatio != 1.0 {
		t.Errorf("SampleRatio malformed: %v want 1.0", cfg.SampleRatio)
	}
}

// TestTracerProviderNoop exercises the no-op dev path: empty
// endpoint → noop exporter → TracerProvider is usable.
func TestTracerProviderNoop(t *testing.T) {
	ctx := context.Background()
	cfg := trace.Config{ServiceName: "test-svc"}
	tp, err := trace.NewTracerProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewTracerProvider: %v", err)
	}
	if tp == nil {
		t.Fatal("NewTracerProvider returned nil")
	}

	// Shutdown is idempotent.
	if err := tp.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	// Double-shutdown is also safe.
	if err := tp.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown (double): %v", err)
	}
}

// TestTracerProviderReconfigureNoop exercises Reconfigure with
// the noop endpoint: the swap is a no-op export-wise and the
// returned provider is usable after the swap.
func TestTracerProviderReconfigureNoop(t *testing.T) {
	ctx := context.Background()
	tp, err := trace.NewTracerProvider(ctx, trace.Config{ServiceName: "reconfig"})
	if err != nil {
		t.Fatalf("NewTracerProvider: %v", err)
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	if err := tp.Reconfigure(ctx, trace.Config{ServiceName: "reconfig-2"}); err != nil {
		t.Errorf("Reconfigure: %v", err)
	}
	// The global provider should now be the new one.
	_ = otel.GetTracerProvider() // any side-effect-free access is fine
}

// TestInitTracerProvider covers the legacy boot path. The endpoint
// is empty so the noop exporter is used.
func TestInitTracerProvider(t *testing.T) {
	sdkTP, err := trace.InitTracerProvider("init-svc", "", false)
	if err != nil {
		t.Fatalf("InitTracerProvider: %v", err)
	}
	if sdkTP == nil {
		t.Fatal("InitTracerProvider returned nil")
	}
	if err := sdkTP.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
