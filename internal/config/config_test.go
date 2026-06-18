package config

import (
	"os"
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
	os.Setenv("PROMPTSHEON_ADDR", ":9090")
	os.Setenv("PROMPTSHEON_DB_PATH", "/tmp/test.db")
	os.Setenv("PROMPTSHEON_LOG_LEVEL", "debug")
	os.Setenv("PROMPTSHEON_AUTH", "false")
	os.Setenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT", "60")
	os.Setenv("PROMPTSHEON_RATE_LIMIT_RATE", "200")
	defer os.Unsetenv("PROMPTSHEON_ADDR")
	defer os.Unsetenv("PROMPTSHEON_DB_PATH")
	defer os.Unsetenv("PROMPTSHEON_LOG_LEVEL")
	defer os.Unsetenv("PROMPTSHEON_AUTH")
	defer os.Unsetenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT")
	defer os.Unsetenv("PROMPTSHEON_RATE_LIMIT_RATE")

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
			os.Setenv("PROMPTSHEON_AUTH", tt.value)
			defer os.Unsetenv("PROMPTSHEON_AUTH")

			cfg := LoadConfig()
			if cfg.Auth != tt.expected {
				t.Errorf("Auth = %v, want %v for value %q", cfg.Auth, tt.expected, tt.value)
			}
		})
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
