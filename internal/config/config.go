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
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Addr:     ":8080",
		DBPath:   "promptsheon.db",
		LogLevel: "info",
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
	if v := os.Getenv("PROMPTSHEON_AUTH"); v == "1" || v == "true" || v == "yes" {
		cfg.Auth = true
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
