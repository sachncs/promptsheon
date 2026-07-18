// Package config manages configuration for the Promptsheon server.
package config

import (
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the server.
type Config struct {
	Addr      string // Listen address (e.g., ":8080")
	DBBackend string // "sqlite" (default) or "postgres"
	DBPath    string // SQLite database file path
	DBDSN     string // Postgres connection string (when DBBackend=postgres)
	LogLevel  string // Log level: debug, info, warn, error
	Auth      bool   // Enable authentication and authorization

	// Server timeouts
	WriteTimeout      int // Write timeout in seconds (default: 30)
	ReadTimeout       int // Read timeout in seconds (default: 30)
	ReadHeaderTimeout int // ReadHeader timeout in seconds (default: 10)
	IdleTimeout       int // Idle timeout in seconds (default: 120)

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

	// Approval policy: "maker_checker" (default) or "majority".
	ApprovalPolicy string
}

const defaultAddr = ":8080"
const valFalse = "false"
const valTrue = "true"
const valYes = "yes"
const valNo = "no"

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Addr:      defaultAddr,
		DBBackend: "sqlite",
		DBPath:    "promptsheon.db",
		LogLevel:  "info",
		Auth:      true,

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

		// Approval policy defaults to maker-checker (creator may not
		// approve their own release; at least one other identity must).
		ApprovalPolicy: "maker_checker",
	}
}

// LoadConfig reads configuration from environment variables, falling back
// to defaults.
func LoadConfig() Config {
	cfg := DefaultConfig()

	cfg.Addr = getEnvString("PROMPTSHEON_ADDR", cfg.Addr)
	cfg.DBBackend = getEnvString("PROMPTSHEON_DB_BACKEND", cfg.DBBackend)
	cfg.DBPath = getEnvString("PROMPTSHEON_DB_PATH", cfg.DBPath)
	cfg.DBDSN = getEnvString("PROMPTSHEON_DB_DSN", cfg.DBDSN)
	cfg.LogLevel = getEnvString("PROMPTSHEON_LOG_LEVEL", cfg.LogLevel)
	cfg.Auth = getEnvBool("PROMPTSHEON_AUTH", cfg.Auth)

	cfg.WriteTimeout = getEnvInt("PROMPTSHEON_SERVER_WRITE_TIMEOUT", cfg.WriteTimeout)
	cfg.ReadTimeout = getEnvInt("PROMPTSHEON_SERVER_READ_TIMEOUT", cfg.ReadTimeout)
	cfg.ReadHeaderTimeout = getEnvInt("PROMPTSHEON_SERVER_READ_HEADER_TIMEOUT", cfg.ReadHeaderTimeout)
	cfg.IdleTimeout = getEnvInt("PROMPTSHEON_SERVER_IDLE_TIMEOUT", cfg.IdleTimeout)

	cfg.RateLimitRate = getEnvInt("PROMPTSHEON_RATE_LIMIT_RATE", cfg.RateLimitRate)
	cfg.RateLimitInterval = getEnvInt("PROMPTSHEON_RATE_LIMIT_INTERVAL", cfg.RateLimitInterval)
	cfg.RateLimitBurst = getEnvInt("PROMPTSHEON_RATE_LIMIT_BURST", cfg.RateLimitBurst)

	cfg.CircuitBreakerFailureThreshold = getEnvInt("PROMPTSHEON_CIRCUIT_BREAKER_FAILURE_THRESHOLD", cfg.CircuitBreakerFailureThreshold)
	cfg.CircuitBreakerSuccessThreshold = getEnvInt("PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD", cfg.CircuitBreakerSuccessThreshold)
	cfg.CircuitBreakerCooldown = getEnvInt("PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN", cfg.CircuitBreakerCooldown)

	cfg.LLMFallback = getEnvString("PROMPTSHEON_LLM_FALLBACK", cfg.LLMFallback)
	cfg.OTelEndpoint = getEnvString("PROMPTSHEON_OTEL_ENDPOINT", cfg.OTelEndpoint)
	cfg.OTelInsecure = getEnvBool("PROMPTSHEON_OTEL_INSECURE", cfg.OTelInsecure)
	cfg.CORSOrigins = getEnvString("PROMPTSHEON_CORS_ORIGINS", cfg.CORSOrigins)
	cfg.ApprovalPolicy = getEnvString("PROMPTSHEON_APPROVAL_POLICY", cfg.ApprovalPolicy)

	sanitizeConfig(&cfg)
	return cfg
}

func getEnvString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	switch v {
	case "1", valTrue, valYes:
		return true
	case "0", valFalse, valNo:
		return false
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("config: invalid "+key+", using default",
			"value", v, "default", defaultVal, "err", err)
		return defaultVal
	}
	return n
}

func sanitizeConfig(cfg *Config) {
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
}

// Port extracts the port number from the address string. The
// implementation uses net.SplitHostPort so IPv6 literals
// (e.g. "[::1]:8080") and empty ports (":http") are handled
// correctly. Returns 8080 if the address can't be parsed.
func (c *Config) Port() int {
	host, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		// No port in the address: try the bare string as a
		// port number for the ":8080" form.
		if strings.HasPrefix(c.Addr, ":") {
			if n, err := strconv.Atoi(c.Addr[1:]); err == nil {
				return n
			}
		}
		// Address like ":http": SplitHostPort returns the
		// service name in port. We don't do service-name
		// resolution; just return the default.
		_ = host
		return 8080
	}
	if n, err := strconv.Atoi(port); err == nil {
		return n
	}
	// Address like ":http" or "host:http": SplitHostPort
	// succeeds but the port is a service name. We could call
	// net.LookupPort here, but that adds a syscall on the
	// hot path; return the default and let the caller
	// override via configuration.
	return 8080
}
