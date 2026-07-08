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
	if cfg.CircuitBreakerFailureThreshold != 5 {
		t.Errorf("CircuitBreakerFailureThreshold = %d, want 5", cfg.CircuitBreakerFailureThreshold)
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
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"empty", "", true},
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
		{":8080", 8080},
		{":3000", 3000},
		{"localhost:8080", 8080},
		{"0.0.0.0:9090", 9090},
		{"invalid", 8080},
		// IPv6 literals — these used to break the simple
		// "scan for last colon" parser. SplitHostPort handles
		// them correctly.
		{"[::1]:8080", 8080},
		{"[2001:db8::1]:9090", 9090},
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
