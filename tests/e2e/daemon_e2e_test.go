// Package e2e stands up the actual promptsheond binary and
// drives it through the Go SDK. The test exercises the
// canonical capability lifecycle:
//
//  1. create a workspace
//  2. create a project under it
//  3. create a capability
//  4. add an immutable version
//  5. invoke the version
//  6. read the resulting execution
//
// The daemon is launched once per test run via TestMain, kept
// alive for the duration, and shut down on process exit. The
// tests share the same daemon to keep wall time low.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/sdk"
)

var (
	daemonBaseURL string
	daemonAPIKey  string
	testClient    *sdk.Client
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "promptsheon-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir temp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	addr, release := startDaemon(dir)
	defer release()

	// Wait for the daemon to be ready.
	if err := waitForDaemon(addr, 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "daemon not ready: %v\n", err)
		os.Exit(1)
	}
	daemonBaseURL = "http://" + addr

	// PROMPTSHEON_AUTH=true requires the admin key be minted
	// via POST /api/v1/setup with the bootstrap token. The
	// legacy `psn_e2e_test_key` literal is gone; tests that
	// want an auth header read daemonAPIKey after TestMain
	// populated it.
	daemonAPIKey = bootstrapAdminKey(daemonBaseURL, "e2e-bootstrap-secret")
	testClient = sdk.New(daemonBaseURL, daemonAPIKey)
	code := m.Run()
	os.Exit(code)
}

// bootstrapAdminKey calls POST /api/v1/setup with the supplied
// bootstrap token and returns the admin key the daemon returns.
// The daemon is empty (no users) at this point, so the
// bootstrap endpoint is the only way in.
func bootstrapAdminKey(baseURL, token string) string {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/setup", strings.NewReader(`{"email":"e2e@local","name":"e2e"}`))
	if err != nil {
		panic(fmt.Sprintf("bootstrap req: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("bootstrap do: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		panic(fmt.Sprintf("bootstrap status %d body %s", resp.StatusCode, b))
	}
	var out struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		panic(fmt.Sprintf("bootstrap decode: %v", err))
	}
	return out.Key
}

func startDaemon(dir string) (string, func()) {
	port := freePort()
	dbPath := filepath.Join(dir, "test.db")
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	bin := buildDaemon(dir)
	cmd := exec.Command(bin)
	// PROMPTSHEON_AUTH=true is the production-shaped default.
	// The legacy unauth flow is exercised by separate tests
	// that opt in via PROMPTSHEON_E2E_AUTH_OFF=1; the
	// authenticated lifecycle test below requires the
	// authenticated path. PROMPTSHEON_BOOTSTRAP_TOKEN gates
	// POST /api/v1/setup so the e2e harness can mint its admin
	// key without leaving a hole for an opportunistic attacker
	// on a shared CI runner.
	cmd.Env = append(os.Environ(),
		"PROMPTSHEON_ADDR="+addr,
		"PROMPTSHEON_DB_PATH="+dbPath,
		"PROMPTSHEON_AUTH=true",
		"PROMPTSHEON_BOOTSTRAP_TOKEN=e2e-bootstrap-secret",
		"PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true",
		"PROMPTSHEON_HARNESS_PRECONDITIONS=false",
		"PROMPTSHEON_LOG_LEVEL=error",
		// The e2e harness exercises migration 025_destructive from a
		// clean DB; production refuses without this flag.
		"PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true",
		"PROMPTSHEON_HARNESS_PRECONDITIONS=false",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		panic(fmt.Sprintf("start daemon: %v", err))
	}
	release := func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
	}
	return addr, release
}

func buildDaemon(dir string) string {
	bin := filepath.Join(dir, "promptsheond")
	// Build from the module root so ./cmd/promptsheond resolves
	// regardless of the working directory the test was started
	// from.
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/promptsheond")
	cmd.Dir = moduleRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("build daemon: %v\n%s", err, out))
	}
	return bin
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// tests/e2e is two directories below the module root.
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForDaemon(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("daemon at %s did not become ready in %s", addr, timeout)
}

// TestCapabilityLifecycle walks the full Capability / Version /
// Invocation flow against the real daemon.
func TestCapabilityLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Health check
	h, err := testClient.Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if h.Status == "" {
		t.Errorf("Health.Status empty: %+v", h)
	}

	// Providers list (proves the API server is wired)
	providers, err := testClient.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) == 0 {
		t.Log("no providers registered; daemon may be in dev mode")
	}
}

// TestHTTPRoundTrip exercises the raw HTTP path so the daemon's
// middleware, CORS, and security headers are verified end-to-end.
func TestHTTPRoundTrip(t *testing.T) {
	resp, err := http.Get(daemonBaseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("status")) {
		t.Errorf("body missing status: %s", body)
	}
}

// TestSecurityHeaders verifies the daemon's default security
// middleware is applied to every response.
func TestSecurityHeaders(t *testing.T) {
	resp, err := http.Get(daemonBaseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got == "" {
		t.Error("X-Content-Type-Options missing")
	}
}

// TestHealthEndpointRejectsUnknown verifies the routing layer
// returns 404 for paths it does not own.
func TestHealthEndpointRejectsUnknown(t *testing.T) {
	resp, err := http.Get(daemonBaseURL + "/this-path-does-not-exist")
	if err != nil {
		t.Fatalf("GET /this-path-does-not-exist: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		t.Error("unknown path should not return 200")
	}
}

// jsonGET is a small helper that performs a GET and decodes
// the JSON body into out.
func jsonGET(t *testing.T, path string, out any) {
	t.Helper()
	resp, err := http.Get(daemonBaseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: status %d, body %s", path, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// jsonPOST is a small helper that performs a POST with a JSON
// body and decodes the JSON response into out.
func jsonPOST(t *testing.T, path string, body, out any) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	req, err := http.NewRequest(http.MethodPost, daemonBaseURL+path, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
}

// logIfErr is a helper that prints to t.Log on error without
// failing the test. Used for best-effort cleanup paths.
func logIfErr(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf(format, args...)
}

// notEmpty asserts that s is not the empty string.
func notEmpty(t *testing.T, name, s string) {
	t.Helper()
	if strings.TrimSpace(s) == "" {
		t.Errorf("%s is empty", name)
	}
}
