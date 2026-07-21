package config

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.DBPath != "promptsheon.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "promptsheon.db")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if !cfg.Auth {
		t.Error("Auth should default to true")
	}
	if cfg.WriteTimeout != 30 {
		t.Errorf("WriteTimeout = %d, want 30", cfg.WriteTimeout)
	}
	if cfg.ReadTimeout != 30 {
		t.Errorf("ReadTimeout = %d, want 30", cfg.ReadTimeout)
	}
	// H-3 fix: ReadHeaderTimeout must default to a non-zero value so
	// the server is not exposed to Slowloris attacks by default.
	if cfg.ReadHeaderTimeout != 10 {
		t.Errorf("ReadHeaderTimeout = %d, want 10 (Slowloris defence)", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 120 {
		t.Errorf("IdleTimeout = %d, want 120", cfg.IdleTimeout)
	}
	if cfg.RateLimitRate != 100 {
		t.Errorf("RateLimitRate = %d, want 100", cfg.RateLimitRate)
	}
	if cfg.RateLimitInterval != 60 {
		t.Errorf("RateLimitInterval = %d, want 60", cfg.RateLimitInterval)
	}
	if cfg.RateLimitBurst != 50 {
		t.Errorf("RateLimitBurst = %d, want 50", cfg.RateLimitBurst)
	}
	if cfg.CircuitBreakerFailureThreshold != 5 {
		t.Errorf("CircuitBreakerFailureThreshold = %d, want 5", cfg.CircuitBreakerFailureThreshold)
	}
	if cfg.CircuitBreakerSuccessThreshold != 3 {
		t.Errorf("CircuitBreakerSuccessThreshold = %d, want 3", cfg.CircuitBreakerSuccessThreshold)
	}
	if cfg.CircuitBreakerCooldown != 30 {
		t.Errorf("CircuitBreakerCooldown = %d, want 30", cfg.CircuitBreakerCooldown)
	}
	if len(cfg.CORSOrigins) != 0 {
		t.Errorf("CORSOrigins = %v, want empty", cfg.CORSOrigins)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("PROMPTSHEON_ADDR", ":9090")
	_ = os.Setenv("PROMPTSHEON_DB_PATH", "/tmp/test.db")
	_ = os.Setenv("PROMPTSHEON_LOG_LEVEL", "debug")
	_ = os.Setenv("PROMPTSHEON_AUTH", "false")
	_ = os.Setenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT", "60")
	_ = os.Setenv("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT", "5")
	_ = os.Setenv("PROMPTSHEON_RATE_LIMIT_RATE", "200")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_ADDR") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_DB_PATH") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_LOG_LEVEL") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_AUTH") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT") }()
	defer func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT_RATE") }()

	cfg := LoadConfig()

	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Auth {
		t.Error("Auth should be false when set to 'false'")
	}
	if cfg.WriteTimeout != 60 {
		t.Errorf("WriteTimeout = %d, want 60", cfg.WriteTimeout)
	}
	if cfg.ReadHeaderTimeout != 5 {
		t.Errorf("ReadHeaderTimeout = %d, want 5", cfg.ReadHeaderTimeout)
	}
	if cfg.RateLimitRate != 200 {
		t.Errorf("RateLimitRate = %d, want 200", cfg.RateLimitRate)
	}
}

func TestLoadConfigAuthValues(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "false", value: "false", expected: false},
		{name: "0", value: "0", expected: false},
		{name: "no", value: "no", expected: false},
		{name: "true", value: "true", expected: true},
		{name: "1", value: "1", expected: true},
		{name: "yes", value: "yes", expected: true},
		{name: "empty", value: "", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("PROMPTSHEON_AUTH", tt.value)
			defer func() { _ = os.Unsetenv("PROMPTSHEON_AUTH") }()

			cfg := LoadConfig()
			if cfg.Auth != tt.expected {
				t.Errorf("Auth = %v, want %v for value %q", cfg.Auth, tt.expected, tt.value)
			}
		})
	}
}

// TestLoadConfig_InvalidNumericWarns pins the M-4 fix: an invalid
// numeric env var must produce a warning (not silent fallback) so
// operators notice when their configuration is being ignored. We
// capture the slog output to verify the warning is emitted with
// the right key/value pairs.
func TestLoadConfig_InvalidNumericWarns(t *testing.T) {
	// Redirect the default slog logger to a buffer for this test.
	prev := slog.Default()
	defer slog.SetDefault(prev)
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	_ = os.Setenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT", "abc")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT") }()
	cfg := LoadConfig()
	if cfg.WriteTimeout != 30 {
		t.Fatalf("expected default 30, got %d", cfg.WriteTimeout)
	}
	if !strings.Contains(buf.String(), "PROMPTSHEON_SERVER_WRITE_TIMEOUT") {
		t.Fatalf("expected warning about PROMPTSHEON_SERVER_WRITE_TIMEOUT, got %q", buf.String())
	}
}

func TestPort(t *testing.T) {
	tests := []struct {
		addr     string
		expected int
	}{
		{addr: ":8080", expected: 8080},
		{addr: ":3000", expected: 3000},
		{addr: "localhost:8080", expected: 8080},
		{addr: "0.0.0.0:9090", expected: 9090},
		{addr: "invalid", expected: 8080},
		// IPv6 literals — these used to break the simple
		// "scan for last colon" parser. SplitHostPort handles
		// them correctly.
		{addr: "[::1]:8080", expected: 8080},
		{addr: "[2001:db8::1]:9090", expected: 9090},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			cfg := Config{Addr: tt.addr}
			if got := cfg.Port(); got != tt.expected {
				t.Errorf("Port() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestPortServiceName(t *testing.T) {
	cfg := Config{Addr: ":http"}
	if got := cfg.Port(); got != 8080 {
		t.Errorf("Port(:http) = %d, want 8080", got)
	}
}

func TestLoadConfig_AdditionalEnvs(t *testing.T) {
	_ = os.Setenv("PROMPTSHEON_RATE_LIMIT_INTERVAL", "30")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT_INTERVAL") }()
	_ = os.Setenv("PROMPTSHEON_RATE_LIMIT_BURST", "100")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT_BURST") }()
	_ = os.Setenv("PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD", "5")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD") }()
	_ = os.Setenv("PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN", "60")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN") }()
	_ = os.Setenv("PROMPTSHEON_OTEL_INSECURE", "true")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_OTEL_INSECURE") }()
	_ = os.Setenv("PROMPTSHEON_CORS_ORIGINS", "*")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_CORS_ORIGINS") }()

	cfg := LoadConfig()
	if cfg.RateLimitInterval != 30 {
		t.Errorf("RateLimitInterval = %d, want 30", cfg.RateLimitInterval)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("RateLimitBurst = %d, want 100", cfg.RateLimitBurst)
	}
	if cfg.CircuitBreakerSuccessThreshold != 5 {
		t.Errorf("CircuitBreakerSuccessThreshold = %d, want 5", cfg.CircuitBreakerSuccessThreshold)
	}
	if cfg.CircuitBreakerCooldown != 60 {
		t.Errorf("CircuitBreakerCooldown = %d, want 60", cfg.CircuitBreakerCooldown)
	}
	if !cfg.OTelInsecure {
		t.Error("OTelInsecure should be true")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("CORSOrigins = %v, want [*]", cfg.CORSOrigins)
	}
}

func TestSanitizeConfigClampsNegative(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	_ = os.Setenv("PROMPTSHEON_RATE_LIMIT_RATE", "-1")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT_RATE") }()
	_ = os.Setenv("PROMPTSHEON_RATE_LIMIT_BURST", "-5")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT_BURST") }()
	_ = os.Setenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT", "-1")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT") }()
	_ = os.Setenv("PROMPTSHEON_SERVER_READ_TIMEOUT", "-1")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_READ_TIMEOUT") }()
	_ = os.Setenv("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT", "-1")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT") }()
	_ = os.Setenv("PROMPTSHEON_SERVER_IDLE_TIMEOUT", "-1")
	defer func() { _ = os.Unsetenv("PROMPTSHEON_SERVER_IDLE_TIMEOUT") }()

	cfg := LoadConfig()
	if cfg.RateLimitRate != 0 {
		t.Errorf("RateLimitRate = %d, want 0", cfg.RateLimitRate)
	}
	if cfg.RateLimitBurst != 0 {
		t.Errorf("RateLimitBurst = %d, want 0", cfg.RateLimitBurst)
	}
	if cfg.WriteTimeout != 30 {
		t.Errorf("WriteTimeout = %d, want 30", cfg.WriteTimeout)
	}
	if cfg.ReadTimeout != 30 {
		t.Errorf("ReadTimeout = %d, want 30", cfg.ReadTimeout)
	}
	if cfg.ReadHeaderTimeout != 10 {
		t.Errorf("ReadHeaderTimeout = %d, want 10", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 120 {
		t.Errorf("IdleTimeout = %d, want 120", cfg.IdleTimeout)
	}
}
