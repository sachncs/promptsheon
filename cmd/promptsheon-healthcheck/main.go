// Command promptsheon-healthcheck polls the daemon's /health
// endpoint and exits 0 on 200, non-zero otherwise. Used as the
// Docker HEALTHCHECK alternative to wget (SEC-CONTAINER-2).
//
//	go build -o promptsheon-healthcheck ./cmd/promptsheon-healthcheck
//
// The binary honours two env vars:
//
//	PROMPTSHEON_HEALTHCHECK_HOST  default "localhost"
//	PROMPTSHEON_HEALTHCHECK_PORT  default "8080"
//
// It also takes an optional single-arg URL override, useful for
// Kubernetes-style readiness probes that want a specific path.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	host := getenv("PROMPTSHEON_HEALTHCHECK_HOST", "localhost")
	portStr := getenv("PROMPTSHEON_HEALTHCHECK_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid port %q: %v\n", portStr, err)
		os.Exit(2)
	}

	path := "/health"
	if len(os.Args) > 1 && os.Args[1] != "" {
		path = os.Args[1]
	}
	url := fmt.Sprintf("http://%s:%d%s", host, port, path)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new request: %v\n", err)
		os.Exit(2)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "health probe failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
