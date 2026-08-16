// healthcheck.go polls the daemon's /health endpoint and exits 0 on 200,
// non-zero otherwise. Used as the Docker HEALTHCHECK alternative to wget
// (SEC-CONTAINER-2).
//
// The binary honours two env vars:
//
//	PROMPTSHEON_HEALTHCHECK_HOST  default "localhost"
//	PROMPTSHEON_HEALTHCHECK_PORT  default "8080"
//
// It also takes an optional single-arg URL override, useful for
// Kubernetes-style readiness probes that want a specific path.
//
// SSRF hardening: the host is restricted to a loopback or
// private-network name (the daemon never exposes /health
// publicly), and the path is restricted to a leading "/" with
// no scheme, host, or fragment. The allowed host list is the
// default localhost plus any IP the operator pins via
// PROMPTSHEON_HEALTHCHECK_HOST; an unrecognised host fails
// closed with exit code 2.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// allowedHealthcheckHosts is the explicit allowlist of host
// values the healthcheck binary will accept. Anything else is
// rejected with exit code 2 to prevent the binary from being
// redirected against a third-party endpoint (SSRF).
var allowedHealthcheckHosts = map[string]struct{}{
	"localhost": {},
	"127.0.0.1": {},
	"::1":       {},
	"":          {}, // empty defaults to localhost in getenv below
}

func runHealthcheck() {
	host := getenv("PROMPTSHEON_HEALTHCHECK_HOST", "localhost")
	if !isAllowedHealthcheckHost(host) {
		fmt.Fprintf(os.Stderr, "healthcheck: host %q is not in the SSRF allowlist (localhost, 127.0.0.1, ::1)\n", host)
		os.Exit(2)
	}
	portStr := getenv("PROMPTSHEON_HEALTHCHECK_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		slog.Error("invalid port", "port", portStr, "err", err)
		os.Exit(2)
	}
	if port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "healthcheck: port %d out of range\n", port)
		os.Exit(2)
	}

	path := "/health"
	if len(os.Args) > 1 && os.Args[1] != "" {
		path = os.Args[1]
	}
	if !isSafeHealthcheckPath(path) {
		fmt.Fprintf(os.Stderr, "healthcheck: path %q must be an absolute local path with no scheme or host\n", path)
		os.Exit(2)
	}

	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   path,
	}
	url := u.String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new request: %v\n", err)
		os.Exit(2)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("health probe failed", "err", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("unhealthy", "status", resp.StatusCode)
		os.Exit(1)
	}
}

// isAllowedHealthcheckHost returns true when host appears in
// the explicit allowlist. The allowlist is small and
// hand-curated; expanding it is a deliberate code change.
func isAllowedHealthcheckHost(host string) bool {
	if _, ok := allowedHealthcheckHosts[host]; ok {
		return true
	}
	return false
}

// isSafeHealthcheckPath validates that path is a safe local
// path: must start with "/", must not start with "//", must not
// contain a scheme separator (":") before any "/", and must
// not contain ".." segments. The check is intentionally
// strict — the binary is the daemon's only allowed caller.
func isSafeHealthcheckPath(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}
	if strings.HasPrefix(path, "//") {
		return false
	}
	if strings.Contains(path, "..") {
		return false
	}
	// Reject a scheme suffix: a path of the form "/x:y" is
	// fine, but "/x://y" is not, because the URL parser
	// would misclassify it. Easier to ban ":" outright
	// for the healthcheck path.
	if strings.Contains(path, ":") {
		return false
	}
	return true
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
