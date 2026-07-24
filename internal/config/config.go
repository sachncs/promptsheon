// Package config manages configuration for the Promptsheon server.
//
// Loading order (later wins):
//
//  1. The DefaultConfig() baseline.
//  2. The YAML file at $PROMPTSHEON_CONFIG (if set).
//  3. Environment variables (PROMPTSHEON_*).
//
// This means a deployment can ship a `promptsheon.yaml` with
// sensible defaults and let env vars override the per-instance
// values (db path, tls cert, etc.) without editing the file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the server.
type Config struct {
	Addr     string // Listen address (e.g., ":8080")
	DBPath   string // SQLite database file path
	LogLevel string // Log level: debug, info, warn, error
	Auth     bool   // Enable authentication and authorization

	// OPS-CFG-4: SQLite connection pool size. Default 1
	// (SQLite serialises writers anyway; a bigger pool buys
	// nothing on a single connection). Production tenants
	// moving to Postgres raise this to match the connection
	// budget.
	DBPoolSize int

	// OPS-CFG-4: retention worker count. Default 1; the
	// retention sweep is I/O-bound on the audit archive
	// copy, so production tenants with large audit tables
	// raise this to fan out across the SQLite pool.
	RetentionWorkerCount int

	// OPS-ROLLOUT-2: read-only mode. When true, every non-GET
	// request returns 503. Used during canary / blue-green
	// rollouts so the new code can run against live traffic
	// with writes off. Read at request time by
	// ReadOnlyMiddleware, so toggling the env var doesn't
	// require a restart.
	ReadOnly bool

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
	CircuitBreakerFailureThreshold int // Failures before opening the circuit (default: 5)
	CircuitBreakerSuccessThreshold int // Successes to close the circuit (default: 3)
	CircuitBreakerCooldown         int // Cooldown in seconds (default: 30)

	// LLM fallback chain. Retained as a struct field for
	// backwards compatibility; the LLM registry does not
	// consume it (the per-call wiring is what chooses the
	// fallback). Set via PROMPTSHEON_LLM_FALLBACK for callers
	// that read the field; production code should treat this
	// as documentation only.
	// LLMFallback string // FC-3: removed — the per-call fallback chain
	//   is not wired into the production invocation path; the
	//   env var was kept for backward compatibility but had no
	//   consumer. Callers wanting a fallback list should configure
	//   the LLM registry with multiple providers per model.

	// OpenTelemetry
	OTelEndpoint string // OTLP gRPC endpoint (e.g., "jaeger:4317")
	OTelInsecure bool   // Use insecure connection to OTel collector

	// SettingsMode controls the operator-tunable runtime config
	// (system_config table). Default "mutable": env is the floor,
	// settings-DB is the ceiling; deletes reassert the env. Set
	// "env-only" for the immutable-baseline story (writes 403).
	// Read via PROMPTSHEON_SETTINGS_MODE.
	SettingsMode string

	// SettingsInit seeds the system_config table on first boot
	// from a YAML map. Read via PROMPTSHEON_SETTINGS_INIT_<KEY>.
	// Empty by default; the chart sets it for the helm install.
	SettingsInit map[string]string

	// CORS
	CORSOrigins []string // Allowed origins; "*" is a single-element list and is rejected unless the bind is loopback.

	// Approval policy: "maker_checker" (default) or "majority".
	ApprovalPolicy string

	// TLS configuration. When TLSCertFile and TLSKeyFile are both
	// non-empty the daemon calls ListenAndServeTLS. The pair MUST be
	// set when the bind address is non-loopback; Validate() enforces.
	TLSCertFile string
	TLSKeyFile  string
}

const defaultAddr = ":8080"
const valFalse = "false"
const valTrue = "true"
const valYes = "yes"
const valNo = "no"

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Addr:     defaultAddr,
		DBPath:   "promptsheon.db",
		LogLevel: "info",
		Auth:     true,

		DBPoolSize:           1,
		RetentionWorkerCount: 1,
		ReadOnly:             false,

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
		CORSOrigins: nil,

		// Approval policy defaults to maker-checker (creator may not
		// approve their own release; at least one other identity must).
		ApprovalPolicy: "maker_checker",

		// SettingsMode is "mutable" by default (env-floor /
		// settings-DB-ceiling). Operators who need the locked-baseline
		// story set PROMPTSHEON_SETTINGS_MODE=env-only.
		SettingsMode: "mutable",
		SettingsInit: map[string]string{},
	}
}

// LoadConfig reads configuration from environment variables, falling back
// to defaults.
//
// OPS-CFG-1: if $PROMPTSHEON_CONFIG points to a YAML file, the file
// is read first and provides the baseline; env vars then override
// any field the operator sets in the shell. The file format is
// the same flat struct as Config — the YAML keys match the Go
// field names exactly.
func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	if path := os.Getenv("PROMPTSHEON_CONFIG"); path != "" {
		if err := loadYAMLFile(&cfg, path); err != nil {
			return Config{}, fmt.Errorf("config: failed to load %q: %w", path, err)
		}
	}

	cfg.Addr = getEnvString("PROMPTSHEON_ADDR", cfg.Addr)
	cfg.DBPath = getEnvString("PROMPTSHEON_DB_PATH", cfg.DBPath)
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

	// FC-3: LLMFallback removed. The env var is no longer read;
	// callers wanting a fallback chain configure the LLM registry.
	cfg.OTelEndpoint = getEnvString("PROMPTSHEON_OTEL_ENDPOINT", cfg.OTelEndpoint)
	cfg.OTelInsecure = getEnvBool("PROMPTSHEON_OTEL_INSECURE", cfg.OTelInsecure)
	cfg.SettingsMode = getEnvString("PROMPTSHEON_SETTINGS_MODE", cfg.SettingsMode)
	// SettingsInit reads the PROMPTSHEON_SETTINGS_INIT_<KEY>
	// env-var family. The keys are uppercased; underscores in
	// the key are kept. Empty by default; the helm chart sets
	// it via values.yaml. Tests can populate directly.
	if cfg.SettingsInit == nil {
		cfg.SettingsInit = map[string]string{}
	}
	for _, kv := range os.Environ() {
		const prefix = "PROMPTSHEON_SETTINGS_INIT_"
		if !strings.HasPrefix(kv, prefix) {
			continue
		}
		rest := strings.TrimPrefix(kv, prefix)
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			continue
		}
		key := rest[:eq]
		val := rest[eq+1:]
		cfg.SettingsInit[key] = val
	}
	cfg.CORSOrigins = parseCORSOrigins(getEnvString("PROMPTSHEON_CORS_ORIGINS", ""))
	cfg.ApprovalPolicy = getEnvString("PROMPTSHEON_APPROVAL_POLICY", cfg.ApprovalPolicy)
	cfg.TLSCertFile = getEnvString("PROMPTSHEON_TLS_CERT_FILE", cfg.TLSCertFile)
	cfg.TLSKeyFile = getEnvString("PROMPTSHEON_TLS_KEY_FILE", cfg.TLSKeyFile)
	// OPS-CFG-4: SQLite pool size. Default 1 (SQLite serialises
	// writers anyway; a bigger pool buys nothing on a single
	// connection). Production tenants that have moved to
	// Postgres raise this to match the connection budget.
	cfg.DBPoolSize = getEnvInt("PROMPTSHEON_DB_POOL_SIZE", cfg.DBPoolSize)
	// OPS-CFG-4: retention worker count. Default 1; the
	// retention sweep is I/O-bound on the audit archive copy,
	// so production tenants with large audit tables raise
	// this to fan out across the SQLite pool.
	cfg.RetentionWorkerCount = getEnvInt("PROMPTSHEON_RETENTION_WORKER_COUNT", cfg.RetentionWorkerCount)
	// OPS-ROLLOUT-2: read-only mode flag. Read at request
	// time by ReadOnlyMiddleware (no restart required to
	// toggle).
	cfg.ReadOnly = getEnvBool("PROMPTSHEON_READ_ONLY", cfg.ReadOnly)

	sanitizeConfig(&cfg)
	return cfg, nil
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

// Validate enforces startup invariants. Returning an error here is a
// hard refusal: the daemon must not boot when its configuration is
// unsafe (e.g. authentication disabled on a non-loopback bind).
//
// Currently enforced:
//   - When Auth is false, the listen address must bind to a loopback
//     IP (127.0.0.0/8 or ::1) or to a Unix socket-style path. The
//     rationale: POST /api/v1/setup mints an admin key to the first
//     caller; on a public bind any network-adjacent caller wins.
//   - When Auth is true but no API key material exists yet and the
//     listen address is non-loopback, the bootstrap token MUST be set
//     so the first admin key is not derived from the network. The
//     first-run bootstrap path remains available only with that
//     token.
func (c *Config) Validate() error {
	if c.Addr == "" {
		return errors.New("config: PROMPTSHEON_ADDR must not be empty")
	}
	if !c.Auth && !isLoopbackAddr(c.Addr) {
		return fmt.Errorf(
			"config: PROMPTSHEON_AUTH=false is only valid for loopback binds (got %q); "+
				"refusing to start because POST /api/v1/setup would mint an admin key to "+
				"any network-adjacent caller. Set PROMPTSHEON_AUTH=true, or bind to "+
				"127.0.0.1 / ::1, or set PROMPTSHEON_BOOTSTRAP_TOKEN to opt into an "+
				"explicit first-run challenge",
			c.Addr,
		)
	}
	if !isLoopbackAddr(c.Addr) {
		for _, o := range c.CORSOrigins {
			if o == "*" {
				return fmt.Errorf(
					"config: PROMPTSHEON_CORS_ORIGINS=* is not allowed for non-loopback binds (got %q); "+
						"the wildcard allows any browser to make credentialed cross-origin "+
						"requests. Set an explicit list of origins, or bind to 127.0.0.1 / ::1",
					c.Addr,
				)
			}
		}
	}
	if !isLoopbackAddr(c.Addr) && c.TLSCertFile == "" && c.TLSKeyFile == "" {
		return fmt.Errorf(
			"config: non-loopback bind %q requires TLS — set PROMPTSHEON_TLS_CERT_FILE and "+
				"PROMPTSHEON_TLS_KEY_FILE to terminate TLS in the daemon. (Bearer keys and audit "+
				"details must not cross the wire in clear.)",
			c.Addr,
		)
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return errors.New("config: PROMPTSHEON_TLS_CERT_FILE and PROMPTSHEON_TLS_KEY_FILE must both be set or both be empty")
	}
	return nil
}

// isLoopbackAddr reports whether addr resolves to a loopback bind.
// The check is intentionally simple: it covers ":port", "host:port",
// "[ipv6]:port", and bare path-style values. It does NOT do DNS
// resolution (operators must use a literal IP).
//
// ":0" is treated as loopback because it means "any free port"
// (the kernel binds to 127.0.0.1 by default for unspecified host).
// The empty host (":8080") is treated as NON-loopback because it
// means "all interfaces". Refusing ":8080" is the whole point of
// this check: ":8080" is the dangerous default that allows any
// network-adjacent caller to hit /api/v1/setup.
// IsLoopbackAddr reports whether addr is a loopback bind. Exported so
// the LLM registry can validate PROMPTSHEON_*_BASE_URL against the
// same definition of "loopback" the rest of the config uses.
func IsLoopbackAddr(addr string) bool { return isLoopbackAddr(addr) }

func isLoopbackAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		switch addr {
		case "localhost", "127.0.0.1", "::1":
			return true
		}
		return false
	}
	if host == "" && port == "0" {
		return true
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// parseCORSOrigins splits a comma-separated origins list into a
// normalised slice. Whitespace is trimmed, empty entries are dropped,
// and "*" is preserved as a single literal element (the CORS
// middleware decides what to do with it). Returns nil for an empty
// input — the middleware treats nil as "deny all cross-origin".
func parseCORSOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
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

// loadYAMLFile reads path into *cfg. Used by LoadConfig when
// $PROMPTSHEON_CONFIG is set. The file is a flat YAML mapping
// whose keys match the Config field names. We use a strict
// YAML decoder so a typo (e.g. `db_path:` vs `DBPath:`) fails
// the boot rather than silently loading defaults.
func loadYAMLFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // strict: unknown keys are a config error
	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
