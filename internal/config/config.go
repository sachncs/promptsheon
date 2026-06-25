// Package config manages configuration for the Promptsheon server.
package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the server.
type Config struct {
	Addr     string // Listen address (e.g., ":8080")
	DBPath   string // SQLite database file path
	LogLevel string // Log level: debug, info, warn, error
	Auth     bool   // Enable authentication and authorization

	// Server timeouts
	WriteTimeout       int // Write timeout in seconds (default: 30)
	ReadTimeout        int // Read timeout in seconds (default: 30)
	ReadHeaderTimeout  int // ReadHeader timeout in seconds (default: 10)
	IdleTimeout        int // Idle timeout in seconds (default: 120)

	// Rate limiting
	RateLimitRate     int // Requests per interval (default: 100)
	RateLimitInterval int // Interval in seconds (default: 60)
	RateLimitBurst    int // Burst capacity (default: 50)

	// Circuit breaker
	CircuitBreakerFailureThreshold int // Failures before opening (default: 5)
	CircuitBreakerSuccessThreshold int // Successes to close (default: 3)
	CircuitBreakerCooldown         int // Cooldown in seconds (default: 30)

	// LLM fallback
	LLMFallback string // Comma-separated fallback providers (e.g., "anthropic,ollama")

	// OpenTelemetry
	OTelEndpoint string // OTLP gRPC endpoint (e.g., "jaeger:4317")
	OTelInsecure bool   // Use insecure connection to OTel collector

	// CORS
	CORSOrigins string // Comma-separated allowed origins, or "*" to allow all (insecure)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Addr:     ":8080",
		DBPath:   "promptsheon.db",
		LogLevel: "info",
		Auth:     true,

		WriteTimeout:      30,
		ReadTimeout:       30,
		ReadHeaderTimeout: 10,
		IdleTimeout:       120,

		RateLimitRate:     100,
		RateLimitInterval: 60,
		RateLimitBurst:    50,

		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerSuccessThreshold: 3,
		CircuitBreakerCooldown:         30,

		// Default CORS policy: deny all cross-origin requests. Operators
		// must explicitly set PROMPTSHEON_CORS_ORIGINS to a list of
		// origins or "*" (for trusted local development only).
		CORSOrigins: "",
	}
}

// LoadConfig reads configuration from environment variables, falling back
// to defaults.
func LoadConfig() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("PROMPTSHEON_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("PROMPTSHEON_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("PROMPTSHEON_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("PROMPTSHEON_AUTH"); v == "0" || v == "false" || v == "no" {
		cfg.Auth = false
	} else if v == "1" || v == "true" || v == "yes" {
		cfg.Auth = true
	}

	if v := os.Getenv("PROMPTSHEON_SERVER_WRITE_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.WriteTimeout = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_SERVER_READ_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReadTimeout = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReadHeaderTimeout = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_SERVER_IDLE_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.IdleTimeout = n
		}
	}

	if v := os.Getenv("PROMPTSHEON_RATE_LIMIT_RATE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitRate = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_RATE_LIMIT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitInterval = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_RATE_LIMIT_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitBurst = n
		}
	}

	if v := os.Getenv("PROMPTSHEON_CIRCUIT_BREAKER_FAILURE_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.CircuitBreakerFailureThreshold = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.CircuitBreakerSuccessThreshold = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.CircuitBreakerCooldown = n
		}
	}

	if v := os.Getenv("PROMPTSHEON_LLM_FALLBACK"); v != "" {
		cfg.LLMFallback = v
	}

	if v := os.Getenv("PROMPTSHEON_OTEL_ENDPOINT"); v != "" {
		cfg.OTelEndpoint = v
	}
	if v := os.Getenv("PROMPTSHEON_OTEL_INSECURE"); v == "true" || v == "1" || v == "yes" {
		cfg.OTelInsecure = true
	}

	if v := os.Getenv("PROMPTSHEON_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = v
	}

	// Sanity-check the numeric config values. Operators can otherwise
	// crash the server with PROMPTSHEON_RATE_LIMIT_RATE=-1 or similar.
	if cfg.RateLimitRate < 0 {
		cfg.RateLimitRate = 0
	}
	if cfg.RateLimitBurst < 0 {
		cfg.RateLimitBurst = 0
	}
	if cfg.WriteTimeout < 0 {
		cfg.WriteTimeout = 30
	}
	if cfg.ReadTimeout < 0 {
		cfg.ReadTimeout = 30
	}
	if cfg.ReadHeaderTimeout < 0 {
		cfg.ReadHeaderTimeout = 10
	}
	if cfg.IdleTimeout < 0 {
		cfg.IdleTimeout = 120
	}

	return cfg
}

// Port extracts the port number from the address string.
func (c Config) Port() int {
	// Simple extraction: find the last colon and parse the number.
	addr := c.Addr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port, err := strconv.Atoi(addr[i+1:])
			if err == nil {
				return port
			}
			break
		}
	}
	return 8080
}
