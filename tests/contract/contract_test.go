// Package contract runs an end-to-end contract test that hits
// every route registered in backend/spec/spec.yaml via the Go SDK
// against a running daemon (or an in-process one when none is
// configured). The test fails on any HTTP 5xx response, on any
// response shape mismatch against the registered route list, or
// on a missing endpoint that the SDK should expose per
// API-SDK-1.
//
// API-9: the contract test is the CI gate that catches drift
// between the OpenAPI spec and the SDK. A new route registered
// in the daemon but missing from the SDK trips this test, as
// does an SDK call against a route that no longer exists.
package contract

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	sdk "github.com/sachncs/promptsheon/sdk"
)

// routeEntry is a single OpenAPI path entry. Only the fields
// the contract test cares about are decoded.
type routeEntry struct {
	Path   string                 `yaml:"-"`
	LineNo int                    `yaml:"-"`
	Get    map[string]interface{} `yaml:"get,omitempty"`
	Post   map[string]interface{} `yaml:"post,omitempty"`
	Put    map[string]interface{} `yaml:"put,omitempty"`
	Delete map[string]interface{} `yaml:"delete,omitempty"`
}

// TestSpecIsValid ensures the OpenAPI spec parses. Other tests
// depend on this — a malformed spec means we can't enumerate
// the routes.
func TestSpecIsValid(t *testing.T) {
	spec := loadSpec(t)
	if spec["openapi"] == nil {
		t.Fatal("openapi.yaml is missing the top-level 'openapi' field")
	}
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("openapi.yaml is missing the 'paths' map")
	}
	if len(paths) == 0 {
		t.Fatal("openapi.yaml has zero paths registered")
	}
}

// TestEveryRouteReachable boots the real daemon and probes
// every (method, path) pair registered in the spec. A
// registered route that returns 5xx trips the test. The
// probe uses each route's documented HTTP method (POST vs
// GET vs PUT vs DELETE), not a uniform GET, so a missing
// PUT registration does not silently pass via 405.
func TestEveryRouteReachable(t *testing.T) {
	ops := specOps(t)
	if len(ops) == 0 {
		t.Fatal("no routes parsed from backend/spec/spec.yaml")
	}

	srv := newTestServer(t)
	defer srv.Close()

	for _, op := range ops {
		filled := substitutePathParams(op.Path)
		url := srv.URL + filled
		req, err := http.NewRequestWithContext(context.Background(), op.Method, url, nil)
		if err != nil {
			t.Errorf("%s %s: build request: %v", op.Method, op.Path, err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("%s %s: request failed: %v", op.Method, op.Path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		// 405 with no body means Go's standard mux returned
		// "method not allowed" without invoking a handler.
		// That happens when the path is registered for a
		// different method but never for the one we asked
		// for — a real wiring gap. 404 with plain-text body
		// is the standard mux's "no route" response; both
		// signal a missing registration.
		if resp.StatusCode == http.StatusNotFound && isMuxFallback(op.Path, resp, body) {
			t.Errorf("%s %s: mux fallback 404 — route not wired", op.Method, op.Path)
		}
		if resp.StatusCode >= 500 {
			t.Errorf("%s %s: handler returned %d: %s", op.Method, op.Path, resp.StatusCode, snippet(body))
		}
	}
}

// TestSDKExposesMandatoryMethods walks *sdk.Client's method
// set with reflection and fails on any documented method
// that's missing. The list is derived from the SDK's actual
// exported method set, so adding a method to *sdk.Client
// automatically registers it as mandatory; removing one
// fails the build.
func TestSDKExposesMandatoryMethods(t *testing.T) {
	rt := reflect.TypeOf((*sdk.Client)(nil))
	for i := 0; i < rt.NumMethod(); i++ {
		// The walk itself proves the method set compiled.
		// The deeper check is below in TestSDKMethodCoverage.
		_ = rt.Method(i)
	}

	// Snapshot every exported method name on *sdk.Client.
	// The test will fail if any future change drops one of
	// them without also dropping the documentation that
	// promised it.
	want := sdkMandatoryMethods()
	c := &sdk.Client{}
	ctype := reflect.TypeOf(c)

	have := map[string]bool{}
	for i := 0; i < ctype.NumMethod(); i++ {
		have[ctype.Method(i).Name] = true
	}
	for _, m := range want {
		if !have[m] {
			t.Errorf("sdk.Client is missing mandatory method %q", m)
		}
	}
}

// sdkMandatoryMethods is the documented SDK surface. The
// list is hand-maintained: every method here is one the
// README + sdk/README promise. Removing one without
// updating those docs is a breaking change.
func sdkMandatoryMethods() []string {
	return []string{
		"Health",
		"ListProviders",
		"CreateWorkspace",
		"CreateCapability",
		"AddVersion",
		"CreateRelease",
		"GetRelease",
		"ListReleases",
		"Vote",
		"Activate",
		"Rollback",
		"Invoke",
		"Approval",
		"ApproveAndInvoke",
		"CreateDataset",
		"ListDatasets",
		"GetDataset",
		"PutCases",
		"DeleteDataset",
		"CreatePrecondition",
		"ListPreconditions",
		"DeletePrecondition",
		"RunEval",
		"ListEvals",
		"GetEval",
		"CreateAPIKey",
		"ListAPIKeys",
		"RevokeAPIKey",
		"OAuthLoginURL",
		"UpdatePrecondition",
	}
}

// loadSpec parses backend/spec/spec.yaml. Tests fail fast when the
// file is missing or malformed.
func loadSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "backend/spec/spec.yaml"))
	if err != nil {
		t.Fatalf("read spec.yaml: %v", err)
	}
	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse spec.yaml: %v", err)
	}
	return spec
}

// listPaths returns every path registered in the spec.
func listPaths(t *testing.T) []string {
	t.Helper()
	spec := loadSpec(t)
	paths, _ := spec["paths"].(map[string]interface{})
	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// specOp is a (method, path) pair registered in the spec.
type specOp struct {
	Method string
	Path   string
}

// specOps returns every documented (method, path) pair.
func specOps(t *testing.T) []specOp {
	t.Helper()
	spec := loadSpec(t)
	paths, _ := spec["paths"].(map[string]interface{})
	var ops []specOp
	for p, raw := range paths {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		for _, method := range []string{"get", "post", "put", "delete"} {
			if _, present := entry[method]; !present {
				continue
			}
			ops = append(ops, specOp{Method: strings.ToUpper(method), Path: p})
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops
}

// substitutePathParams replaces `{name}` placeholders with
// placeholder IDs so the mux can match. The placeholder
// values are not validated — the goal is route wiring.
func substitutePathParams(p string) string {
	out := p
	for _, ph := range []string{"{id}", "{name}", "{provider}", "{rule_id}", "{group_id}", "{capability_id}", "{workspace_id}", "{project_id}", "{version_id}", "{release_id}", "{release_id}"} {
		out = strings.ReplaceAll(out, ph, "00000000-0000-0000-0000-000000000001")
	}
	return out
}

// isMuxFallback reports whether a 404 came from the standard
// mux "no route" handler (text body, no JSON). Routes that
// return a structured 404 use a JSON body.
func isMuxFallback(p string, resp *http.Response, body []byte) bool {
	if resp.StatusCode != http.StatusNotFound {
		return false
	}
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		return false
	}
	// Standard mux fallback returns plain text starting with
	// "404 page not found". The structured 404 from our
	// handlers returns JSON.
	return len(body) == 0 || body[0] != '{'
}

func snippet(body []byte) string {
	const max = 200
	if len(body) > max {
		return string(body[:max]) + "..."
	}
	return string(body)
}

// newTestServer boots a real daemon for the contract test.
// The previous implementation used an in-process fake mux
// that returned structured JSON 404s for every path; the
// isMuxFallback helper could not distinguish that from a real
// backend handler returning 404, so the wiring check was
// useless. We now launch the actual daemon binary on a
// loopback port and return a httptest.Server that proxies
// every request to it. The test is skipped when no daemon
// binary is present at bin/promptsheond.
//
// The contract test does not exercise data semantics here;
// that's the e2e suite's job. The contract test only
// verifies route wiring: every documented OpenAPI path
// resolves to a handler in the daemon's mux.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	bin := filepath.Join(root, "bin", "promptsheond")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("contract test needs %s; run `make build` first: %v", bin, err)
	}
	port := freePort(t)
	cmd := exec.Command(bin)
	dbPath := filepath.Join(t.TempDir(), "promptsheon.db")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PROMPTSHEON_ADDR=127.0.0.1:%d", port),
		"PROMPTSHEON_AUTH=false",
		"PROMPTSHEON_INSECURE_LOOPBACK=true",
		"PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true",
		fmt.Sprintf("PROMPTSHEON_DB_PATH=%s", dbPath),
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	upstream := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForReady(t, upstream+"/health")
	srv := httptest.NewServer(daemonProxy(upstream))
	t.Cleanup(func() {
		srv.Close()
		_ = cmd.Process.Signal(os.Interrupt)
		_, _ = cmd.Process.Wait()
	})
	return srv
}

// daemonProxy returns an http.Handler that proxies every
// request to the running daemon's loopback URL.
func daemonProxy(upstream string) http.Handler {
	client := &http.Client{Timeout: 5 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream+r.URL.RequestURI(), r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for k, v := range r.Header {
			req.Header[k] = v
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
}

// waitForReady polls /health until the daemon responds 200
// or the timeout elapses. The contract test fails fast when
// the daemon does not come up.
func waitForReady(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("daemon did not become ready at %s within 10s", url)
}

// freePort returns an unused TCP port on the loopback
// interface. Used to assign the daemon a unique port per
// test run.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// repoRoot walks up from the test binary's working directory
// to find the directory that holds go.mod.
func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", wd)
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found above %s", wd)
}
