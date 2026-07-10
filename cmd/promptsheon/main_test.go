package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/promptsheon"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1536 * 1024, "1.5 MiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseTelemetry(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_TELEMETRY")
	defer func() { _ = os.Setenv("PROMPTSHEON_TELEMETRY", orig) }()

	_ = os.Unsetenv("PROMPTSHEON_TELEMETRY")
	m, err := parseTelemetry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Fatalf("expected nil for unset env, got %v", m)
	}

	_ = os.Setenv("PROMPTSHEON_TELEMETRY", "")
	m, err = parseTelemetry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Fatalf("expected nil for empty env, got %v", m)
	}

	_ = os.Setenv("PROMPTSHEON_TELEMETRY", `{"accuracy":0.95,"latency_ms":200}`)
	m, err = parseTelemetry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["accuracy"] != float64(0.95) {
		t.Errorf("expected accuracy=0.95, got %v", m["accuracy"])
	}
	if m["latency_ms"] != float64(200) {
		t.Errorf("expected latency_ms=200, got %v", m["latency_ms"])
	}

	_ = os.Setenv("PROMPTSHEON_TELEMETRY", "not valid json{{")
	m, err = parseTelemetry()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "PROMPTSHEON_TELEMETRY") {
		t.Errorf("error should mention env var name, got: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil map on error, got %v", m)
	}
}

func TestUsageErrorf(t *testing.T) {
	err := usageErrorf("promptsheon hash-object <data>")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(err, errUsage) {
		t.Error("expected errUsage in error chain")
	}
	if !strings.Contains(err.Error(), "promptsheon hash-object <data>") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestServerURL(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_SERVER")
	defer func() { _ = os.Setenv("PROMPTSHEON_SERVER", orig) }()

	_ = os.Unsetenv("PROMPTSHEON_SERVER")
	u := serverURL()
	if u != "http://localhost:8080" {
		t.Errorf("expected default http://localhost:8080, got %q", u)
	}

	_ = os.Setenv("PROMPTSHEON_SERVER", "http://example.com:9090")
	u = serverURL()
	if u != "http://example.com:9090" {
		t.Errorf("expected http://example.com:9090, got %q", u)
	}

	_ = os.Setenv("PROMPTSHEON_SERVER", "http://example.com/")
	u = serverURL()
	if u != "http://example.com" {
		t.Errorf("expected http://example.com (no trailing slash), got %q", u)
	}
}

func TestHandleEarlyExit(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"promptsheon", "init"}
	if handleEarlyExit() {
		t.Error("handleEarlyExit should return false for 'init'")
	}

	os.Args = []string{"promptsheon", "--help"}
	out := captureStdout(t, func() {
		if !handleEarlyExit() {
			t.Error("handleEarlyExit should return true for --help")
		}
	})
	if !strings.Contains(out, "init") {
		t.Errorf("--help should print usage containing 'init', got: %s", out[:50])
	}

	os.Args = []string{"promptsheon", "-h"}
	if !handleEarlyExit() {
		t.Error("handleEarlyExit should return true for -h")
	}

	os.Args = []string{"promptsheon", "-help"}
	if !handleEarlyExit() {
		t.Error("handleEarlyExit should return true for -help")
	}

	os.Args = []string{"promptsheon", "--version"}
	out = captureStdout(t, func() {
		if !handleEarlyExit() {
			t.Error("handleEarlyExit should return true for --version")
		}
	})
	if !strings.Contains(out, "promptsheon") {
		t.Errorf("--version should print version containing 'promptsheon', got: %s", out[:50])
	}

	os.Args = []string{"promptsheon", "-V"}
	if !handleEarlyExit() {
		t.Error("handleEarlyExit should return true for -V")
	}

	os.Args = []string{"promptsheon", "-version"}
	if !handleEarlyExit() {
		t.Error("handleEarlyExit should return true for -version")
	}
}

func TestPrintUsage(t *testing.T) {
	out := captureStdout(t, func() { printUsage() })
	if !strings.Contains(out, "init") {
		t.Error("usage output should contain 'init'")
	}
	if !strings.Contains(out, "graph") {
		t.Error("usage output should contain 'graph'")
	}
	if !strings.Contains(out, "workspace") {
		t.Error("usage output should contain 'workspace'")
	}
	if !strings.Contains(out, "capability") {
		t.Error("usage output should contain 'capability'")
	}
}

func TestWalkChain(t *testing.T) {
	nodeMap := map[string]*promptsheon.GraphNode{
		"c1": {Hash: "c1", Parents: []string{"b1"}},
		"b1": {Hash: "b1", Parents: []string{"a1"}},
		"a1": {Hash: "a1", Parents: []string{}},
	}

	t.Run("empty hash", func(t *testing.T) {
		columns := make(map[string]int)
		walkChain("", 0, columns, nodeMap)
		if len(columns) != 0 {
			t.Errorf("expected no columns, got %v", columns)
		}
	})

	t.Run("basic walk", func(t *testing.T) {
		columns := make(map[string]int)
		walkChain("c1", 0, columns, nodeMap)
		if columns["c1"] != 0 || columns["b1"] != 0 || columns["a1"] != 0 {
			t.Errorf("expected all in column 0, got %v", columns)
		}
	})

	t.Run("already assigned returns early", func(t *testing.T) {
		columns := map[string]int{"b1": 5}
		walkChain("c1", 0, columns, nodeMap)
		if columns["c1"] != 0 {
			t.Errorf("expected c1 in col 0, got col %d", columns["c1"])
		}
		if columns["b1"] != 5 {
			t.Errorf("expected b1 to keep col 5, got col %d", columns["b1"])
		}
	})

	t.Run("node not in map", func(t *testing.T) {
		columns := make(map[string]int)
		nm := map[string]*promptsheon.GraphNode{
			"x1": {Hash: "x1", Parents: []string{"missing"}},
		}
		walkChain("x1", 0, columns, nm)
		if columns["x1"] != 0 {
			t.Errorf("expected x1 in col 0, got %v", columns["x1"])
		}
		if columns["missing"] != 0 {
			t.Errorf("expected missing to be assigned col 0 since walkChain assigns before lookup, got %v", columns["missing"])
		}
	})

	t.Run("node with no parents", func(t *testing.T) {
		columns := make(map[string]int)
		nm := map[string]*promptsheon.GraphNode{
			"root": {Hash: "root", Parents: []string{}},
		}
		walkChain("root", 2, columns, nm)
		if columns["root"] != 2 {
			t.Errorf("expected root in col 2, got %v", columns["root"])
		}
	})
}

func TestAssignGraphColumns(t *testing.T) {
	t.Run("empty nodes", func(t *testing.T) {
		cols := assignGraphColumns(nil, nil, "")
		if len(cols) != 0 {
			t.Errorf("expected empty map, got %v", cols)
		}
	})

	t.Run("single branch with HEAD", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "a1", Parents: []string{}},
			{Hash: "b1", Parents: []string{"a1"}},
			{Hash: "c1", Parents: []string{"b1"}, Branches: []string{"main"}, IsHEAD: true},
		}
		refs := []*promptsheon.RefDetail{
			{Name: "main", Hash: "c1"},
		}
		cols := assignGraphColumns(nodes, refs, "main")
		if cols["a1"] != 0 || cols["b1"] != 0 || cols["c1"] != 0 {
			t.Errorf("expected all in col 0, got %v", cols)
		}
	})

	t.Run("multi branch", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "a1", Parents: []string{}},
			{Hash: "b1", Parents: []string{"a1"}},
			{Hash: "c1", Parents: []string{"b1"}, Branches: []string{"main"}, IsHEAD: true},
			{Hash: "d1", Parents: []string{"a1"}, Branches: []string{"feature"}},
		}
		refs := []*promptsheon.RefDetail{
			{Name: "main", Hash: "c1"},
			{Name: "feature", Hash: "d1"},
		}
		cols := assignGraphColumns(nodes, refs, "main")
		if cols["a1"] != 0 || cols["b1"] != 0 || cols["c1"] != 0 {
			t.Errorf("expected main chain in col 0, got a1=%d b1=%d c1=%d", cols["a1"], cols["b1"], cols["c1"])
		}
		if cols["d1"] != 1 {
			t.Errorf("expected feature branch in col 1, got %d", cols["d1"])
		}
	})

	t.Run("fallback to IsHEAD when no ref matches", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "a1", Parents: []string{}},
			{Hash: "b1", Parents: []string{"a1"}, IsHEAD: true},
		}
		refs := []*promptsheon.RefDetail{}
		cols := assignGraphColumns(nodes, refs, "")
		if cols["a1"] != 0 || cols["b1"] != 0 {
			t.Errorf("expected both in col 0, got %v", cols)
		}
	})

	t.Run("unassigned nodes get col 0", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "a1", Parents: []string{}},
			{Hash: "orphan", Parents: []string{}},
		}
		cols := assignGraphColumns(nodes, nil, "")
		if cols["a1"] != 0 || cols["orphan"] != 0 {
			t.Errorf("expected all in col 0, got %v", cols)
		}
	})

	t.Run("two branches feature and dev", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "a1", Parents: []string{}},
			{Hash: "b1", Parents: []string{"a1"}},
			{Hash: "c1", Parents: []string{"b1"}, Branches: []string{"main"}, IsHEAD: true},
			{Hash: "d1", Parents: []string{"a1"}, Branches: []string{"feature"}},
			{Hash: "e1", Parents: []string{"a1"}, Branches: []string{"dev"}},
		}
		refs := []*promptsheon.RefDetail{
			{Name: "main", Hash: "c1"},
			{Name: "feature", Hash: "d1"},
			{Name: "dev", Hash: "e1"},
		}
		cols := assignGraphColumns(nodes, refs, "main")
		if cols["d1"] != 1 {
			t.Errorf("expected feature in col 1, got %d", cols["d1"])
		}
		if cols["e1"] != 2 {
			t.Errorf("expected dev in col 2, got %d", cols["e1"])
		}
	})
}

func TestDrawConnLines(t *testing.T) {
	t.Run("prevCol < col (branch right)", func(t *testing.T) {
		var buf bytes.Buffer
		drawConnLines(&buf, 0, 1, 1, []int{1, 0}, nil)
		want := "| \\ \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("prevCol > col (branch left)", func(t *testing.T) {
		var buf bytes.Buffer
		drawConnLines(&buf, 1, 0, 1, []int{0, 0}, nil)
		want := "/ | \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("prevCol == col (no change)", func(t *testing.T) {
		var buf bytes.Buffer
		drawConnLines(&buf, 0, 0, 0, []int{1}, nil)
		want := "| \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("colRemaining in between", func(t *testing.T) {
		var buf bytes.Buffer
		drawConnLines(&buf, 0, 2, 2, []int{1, 1, 0}, nil)
		want := "| | \\ \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestDrawMergeForkLines(t *testing.T) {
	t.Run("single parent - no-op", func(t *testing.T) {
		var buf bytes.Buffer
		node := &promptsheon.GraphNode{Hash: "c1", Parents: []string{"b1"}}
		drawMergeForkLines(&buf, node, map[string]int{"c1": 0, "b1": 0}, 0, []int{0})
		if buf.Len() != 0 {
			t.Errorf("expected no output for single parent, got %q", buf.String())
		}
	})

	t.Run("merge from different column", func(t *testing.T) {
		var buf bytes.Buffer
		node := &promptsheon.GraphNode{Hash: "c1", Parents: []string{"b1", "d1"}}
		columns := map[string]int{"c1": 0, "b1": 0, "d1": 1}
		drawMergeForkLines(&buf, node, columns, 1, []int{0, 0})
		want := "  \\ \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("merge from three parent columns", func(t *testing.T) {
		var buf bytes.Buffer
		node := &promptsheon.GraphNode{Hash: "e1", Parents: []string{"a1", "b1", "c1"}}
		columns := map[string]int{"e1": 0, "a1": 0, "b1": 1, "c1": 2}
		drawMergeForkLines(&buf, node, columns, 2, []int{0, 0, 0})
		want := "  | / \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("merge with nodes below", func(t *testing.T) {
		var buf bytes.Buffer
		node := &promptsheon.GraphNode{Hash: "c1", Parents: []string{"a1", "b1"}}
		columns := map[string]int{"c1": 0, "a1": 0, "b1": 1}
		drawMergeForkLines(&buf, node, columns, 1, []int{1, 1})
		want := "| \\ \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("fork from different column", func(t *testing.T) {
		var buf bytes.Buffer
		node := &promptsheon.GraphNode{Hash: "a1", Parents: []string{"b1", "c1"}}
		columns := map[string]int{"a1": 1, "b1": 0, "c1": 1}
		drawMergeForkLines(&buf, node, columns, 1, []int{0, 0})
		want := "\\   \n"
		if got := buf.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestRenderGraph(t *testing.T) {
	t.Run("empty nodes", func(t *testing.T) {
		out := renderGraph(nil, nil, 0)
		if out != "" {
			t.Errorf("expected empty string, got %q", out)
		}
	})

	t.Run("single node", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "abcdef1234567890123456789012345678901234", Parents: []string{}, Message: "initial"},
		}
		columns := map[string]int{"abcdef1234567890123456789012345678901234": 0}
		out := renderGraph(nodes, columns, 0)
		if !strings.Contains(out, "o ") {
			t.Error("expected 'o ' marker in output")
		}
		if !strings.Contains(out, "abcdef123456") {
			t.Error("expected hash in output")
		}
		if !strings.Contains(out, "initial") {
			t.Error("expected message in output")
		}
	})

	t.Run("HEAD with branch labels", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Parents: []string{}, Branches: []string{"main"}, IsHEAD: true, Message: "root"},
		}
		columns := map[string]int{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 0}
		out := renderGraph(nodes, columns, 0)
		if !strings.Contains(out, "HEAD -> main") {
			t.Errorf("expected HEAD -> main label, got: %s", out)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		nodes := []*promptsheon.GraphNode{
			{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Parents: []string{}, Branches: []string{}, IsHEAD: true, Message: "detached"},
		}
		columns := map[string]int{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 0}
		out := renderGraph(nodes, columns, 0)
		if !strings.Contains(out, "HEAD") {
			t.Errorf("expected HEAD label, got: %s", out)
		}
	})

	t.Run("linear chain", func(t *testing.T) {
		hashA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		hashB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		hashC := "cccccccccccccccccccccccccccccccccccccccccccc"
		nodes := []*promptsheon.GraphNode{
			{Hash: hashA, Parents: []string{}},
			{Hash: hashB, Parents: []string{hashA}},
			{Hash: hashC, Parents: []string{hashB}, Branches: []string{"main"}, IsHEAD: true, Message: "third"},
		}
		columns := map[string]int{hashA: 0, hashB: 0, hashC: 0}
		out := renderGraph(nodes, columns, 0)
		if !strings.Contains(out, hashA[:12]) {
			t.Errorf("expected %s in output", hashA[:12])
		}
		if !strings.Contains(out, hashB[:12]) {
			t.Errorf("expected %s in output", hashB[:12])
		}
		if !strings.Contains(out, "third") {
			t.Errorf("expected 'third' message in output")
		}
	})

	t.Run("branch with column transition", func(t *testing.T) {
		hashA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		hashB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		hashC := "cccccccccccccccccccccccccccccccccccccccccccc"
		hashD := "dddddddddddddddddddddddddddddddddddddddddddd"
		nodes := []*promptsheon.GraphNode{
			{Hash: hashA, Parents: []string{}},
			{Hash: hashB, Parents: []string{hashA}},
			{Hash: hashC, Parents: []string{hashB}, Branches: []string{"main"}, IsHEAD: true, Message: "third"},
			{Hash: hashD, Parents: []string{hashA}, Branches: []string{"feature"}},
		}
		columns := map[string]int{hashA: 0, hashB: 0, hashC: 0, hashD: 1}
		out := renderGraph(nodes, columns, 1)
		if !strings.Contains(out, "\\") {
			t.Errorf("expected backslash connection line in output, got:\n%s", out)
		}
	})

	t.Run("merge commit", func(t *testing.T) {
		hashA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		hashB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		hashC := "cccccccccccccccccccccccccccccccccccccccccccc"
		nodes := []*promptsheon.GraphNode{
			{Hash: hashA, Parents: []string{}},
			{Hash: hashB, Parents: []string{hashA}},
			{Hash: hashC, Parents: []string{hashB, hashA}, Branches: []string{"main"}, IsHEAD: true, Message: "merge"},
		}
		columns := map[string]int{hashA: 0, hashB: 0, hashC: 0}
		out := renderGraph(nodes, columns, 0)
		if !strings.Contains(out, "merge") {
			t.Errorf("expected merge commit message, got:\n%s", out)
		}
	})
}

func TestHTTPGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test"}`))
	}))
	defer server.Close()

	var result map[string]string
	err := httpGet(server.URL+"/api/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result)
	}
}

func TestHTTPGetError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	var result any
	err := httpGet(server.URL+"/api/test", &result)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestHTTPGetNonJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("plain text error"))
	}))
	defer server.Close()

	var result any
	err := httpGet(server.URL+"/api/test", &result)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestHTTPPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","name":"test"}`))
	}))
	defer server.Close()

	var result map[string]string
	err := httpPost(server.URL+"/api/create", map[string]string{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "123" || result["name"] != "test" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestHTTPPostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	var result any
	err := httpPost(server.URL+"/api/create", map[string]string{"name": "test"}, &result)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request' in error, got: %v", err)
	}
}

func TestHTTPPostNonJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer server.Close()

	var result any
	err := httpPost(server.URL+"/api/create", map[string]string{"name": "test"}, &result)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got: %v", err)
	}
}

func TestHTTPDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := httpDelete(server.URL + "/api/delete/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPDeleteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	err := httpDelete(server.URL + "/api/delete/999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestDispatchCommand(t *testing.T) {
	t.Run("help command", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("help", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "init") {
			t.Error("help should print usage containing 'init'")
		}
	})

	t.Run("graph command requires repo", func(t *testing.T) {
		err := dispatchCommand("graph", nil)
		if err == nil {
			t.Fatal("expected error for uninitialized repo")
		}
		if !strings.Contains(err.Error(), "not a promptsheon") {
			t.Errorf("expected repo not initialized error, got: %v", err)
		}
	})

	t.Run("status command requires repo", func(t *testing.T) {
		err := dispatchCommand("status", nil)
		if err == nil {
			t.Fatal("expected error for uninitialized repo")
		}
		if !strings.Contains(err.Error(), "not a promptsheon") {
			t.Errorf("expected repo not initialized error, got: %v", err)
		}
	})

	t.Run("stats command requires repo", func(t *testing.T) {
		err := dispatchCommand("stats", nil)
		if err == nil {
			t.Fatal("expected error for uninitialized repo")
		}
		if !strings.Contains(err.Error(), "not a promptsheon") {
			t.Errorf("expected repo not initialized error, got: %v", err)
		}
	})

	t.Run("verify command requires repo", func(t *testing.T) {
		err := dispatchCommand("verify", nil)
		if err == nil {
			t.Fatal("expected error for uninitialized repo")
		}
		if !strings.Contains(err.Error(), "not a promptsheon") {
			t.Errorf("expected repo not initialized error, got: %v", err)
		}
	})

	t.Run("init command succeeds", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "promptsheon-test-init-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
		origWd, _ := os.Getwd()
		_ = os.Chdir(dir)
		defer func() { _ = os.Chdir(origWd) }()

		out := captureStdout(t, func() {
			err := dispatchCommand("init", nil)
			if err != nil {
				t.Fatalf("init failed: %v", err)
			}
		})
		if !strings.Contains(out, "initialized") {
			t.Errorf("expected 'initialized' in output, got: %s", out)
		}
	})

	t.Run("hash-object", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("hash-object", []string{"test data"})
			if err != nil {
				t.Fatalf("hash-object failed: %v", err)
			}
		})
		out = strings.TrimSpace(out)
		if len(out) != 64 {
			t.Errorf("expected 64-char hash, got %q (len=%d)", out, len(out))
		}
	})

	t.Run("hash-object missing arg", func(t *testing.T) {
		err := dispatchCommand("hash-object", nil)
		if err == nil {
			t.Fatal("expected error for missing arg")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("commit missing arg", func(t *testing.T) {
		err := dispatchCommand("commit", nil)
		if err == nil {
			t.Fatal("expected error for missing arg")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("diff missing args", func(t *testing.T) {
		err := dispatchCommand("diff", nil)
		if err == nil {
			t.Fatal("expected error for missing args")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("branch list empty returns empty", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("branch", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if out != "" {
			t.Errorf("expected no output for empty branch list, got: %q", out)
		}
	})
}

func TestDispatchCommandProvider(t *testing.T) {
	t.Run("provider missing subcommand", func(t *testing.T) {
		err := dispatchCommand("provider", nil)
		if err == nil {
			t.Fatal("expected error for missing subcommand")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("provider unknown subcommand", func(t *testing.T) {
		err := dispatchCommand("provider", []string{"unknown"})
		if err == nil {
			t.Fatal("expected error for unknown subcommand")
		}
		if !strings.Contains(err.Error(), "unknown provider subcommand") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestDispatchCommandWorkspace(t *testing.T) {
	t.Run("workspace missing subcommand", func(t *testing.T) {
		err := dispatchCommand("workspace", nil)
		if err == nil {
			t.Fatal("expected error for missing subcommand")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("workspace unknown subcommand", func(t *testing.T) {
		err := dispatchCommand("workspace", []string{"unknown"})
		if err == nil {
			t.Fatal("expected error for unknown subcommand")
		}
		if !strings.Contains(err.Error(), "unknown workspace subcommand") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestDispatchCommandProject(t *testing.T) {
	t.Run("project missing subcommand", func(t *testing.T) {
		err := dispatchCommand("project", nil)
		if err == nil {
			t.Fatal("expected error for missing subcommand")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("project unknown subcommand", func(t *testing.T) {
		err := dispatchCommand("project", []string{"unknown"})
		if err == nil {
			t.Fatal("expected error for unknown subcommand")
		}
		if !strings.Contains(err.Error(), "unknown project subcommand") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("project list missing workspace id", func(t *testing.T) {
		err := dispatchCommand("project", []string{"list"})
		if err == nil {
			t.Fatal("expected error for missing workspace id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})
}

func TestDispatchCommandCapability(t *testing.T) {
	t.Run("capability missing subcommand", func(t *testing.T) {
		err := dispatchCommand("capability", nil)
		if err == nil {
			t.Fatal("expected error for missing subcommand")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("capability unknown subcommand", func(t *testing.T) {
		err := dispatchCommand("capability", []string{"unknown"})
		if err == nil {
			t.Fatal("expected error for unknown subcommand")
		}
		if !strings.Contains(err.Error(), "unknown capability subcommand") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHandleCmdErrorStderr(t *testing.T) {
	t.Run("errUsage prints message and hint", func(t *testing.T) {
		// handleCmdError calls os.Exit, so we test via subprocess.
		// Instead, we verify the output format by checking that
		// the error message contains all expected pieces.
		err := usageErrorf("promptsheon commit <tree>")
		errMsg := err.Error()
		if !strings.Contains(errMsg, "invalid arguments") {
			t.Errorf("expected 'invalid arguments', got %q", errMsg)
		}
		if !strings.Contains(errMsg, "promptsheon commit <tree>") {
			t.Errorf("expected usage hint, got %q", errMsg)
		}
	})
}

func TestServerURLWithHTTPServer(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_SERVER")
	defer func() { _ = os.Setenv("PROMPTSHEON_SERVER", orig) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	_ = os.Setenv("PROMPTSHEON_SERVER", server.URL)
	u := serverURL()
	if u != strings.TrimRight(server.URL, "/") {
		t.Errorf("expected %s, got %s", strings.TrimRight(server.URL, "/"), u)
	}
}

func TestReadObjectUninitialized(t *testing.T) {
	err := dispatchCommand("read-object", []string{"abc123"})
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}

func TestCatFileUninitialized(t *testing.T) {
	err := dispatchCommand("cat-file", []string{"abc123"})
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}

func TestShowUninitialized(t *testing.T) {
	err := dispatchCommand("show", []string{"abc123"})
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}

func TestLsTreeUninitialized(t *testing.T) {
	err := dispatchCommand("ls-tree", []string{"abc123"})
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}

func TestWriteObjectMissingArg(t *testing.T) {
	err := dispatchCommand("write-object", nil)
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("expected errUsage, got %T: %v", err, err)
	}
}

func TestCheckoutMissingArg(t *testing.T) {
	err := dispatchCommand("checkout", nil)
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("expected errUsage, got %T: %v", err, err)
	}
}

func TestDeleteBranchMissingArg(t *testing.T) {
	err := dispatchCommand("delete-branch", nil)
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("expected errUsage, got %T: %v", err, err)
	}
}

func TestRunMissingPrompt(t *testing.T) {
	err := dispatchCommand("run", nil)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("expected errUsage, got %T: %v", err, err)
	}
}

func TestRunHelpCommandReturnsNil(t *testing.T) {
	err := dispatchCommand("help", nil)
	if err != nil {
		t.Errorf("help command should not fail: %v", err)
	}
}

func TestHTTPPostMarshalError(t *testing.T) {
	err := httpPost("http://example.com", make(chan int), nil)
	if err == nil {
		t.Fatal("expected error for unmarshalable body")
	}
}

func TestHTTPGetConnectionError(t *testing.T) {
	var result any
	err := httpGet("http://127.0.0.1:1", &result)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHTTPPostConnectionError(t *testing.T) {
	var result any
	err := httpPost("http://127.0.0.1:1", map[string]string{"a": "b"}, &result)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHTTPDeleteConnectionError(t *testing.T) {
	err := httpDelete("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

// withTempRepo creates a temporary directory, initialises a
// .promptsheon repository in it with two commits on "main",
// chdirs there, and returns a cleanup function.
func withTempRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "promptsheon-test-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	origWd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() {
		_ = os.Chdir(origWd)
		_ = os.RemoveAll(dir)
	})

	if e := promptsheon.Init(); e != nil {
		t.Fatalf("Init(): %v", e)
	}

	b1 := promptsheon.NewBlobObject("system prompt: You are a helpful assistant")
	b2 := promptsheon.NewBlobObject("tool spec: read_file, execute_command")
	b3 := promptsheon.NewBlobObject("hyperparams: temp=0.7, max_tokens=2048")
	h1, _ := promptsheon.WriteObject(b1)
	h2, _ := promptsheon.WriteObject(b2)
	h3, _ := promptsheon.WriteObject(b3)

	tree := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "system-prompt", Type: promptsheon.TypeBlob, Hash: h1},
		{Name: "tool-spec", Type: promptsheon.TypeBlob, Hash: h2},
		{Name: "hyperparams", Type: promptsheon.TypeBlob, Hash: h3},
	})
	treeHash, err := promptsheon.WriteObject(tree)
	if err != nil {
		t.Fatalf("WriteObject(tree): %v", err)
	}

	tel := map[string]any{"accuracy": 0.95, "latency_ms": 200}
	_, err = promptsheon.Commit(treeHash, nil, "engineer", "Initial agent configuration", tel)
	if err != nil {
		t.Fatalf("Commit(): %v", err)
	}

	return dir
}

func TestCmdInitAlreadyInitialized(t *testing.T) {
	_ = withTempRepo(t)
	err := dispatchCommand("init", nil)
	if err == nil {
		t.Fatal("expected error for already initialized repo")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdWriteObjectWithRepo(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("write-object", []string{"test data"})
		if err != nil {
			t.Fatalf("write-object failed: %v", err)
		}
	})
	out = strings.TrimSpace(out)
	if len(out) != 64 {
		t.Errorf("expected 64-char hash, got %q (len=%d)", out, len(out))
	}
}

func TestCmdReadObjectWithRepo(t *testing.T) {
	dir := withTempRepo(t)

	obj := promptsheon.NewBlobObject("test content")
	hash, err := promptsheon.WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}

	_ = dir
	out := captureStdout(t, func() {
		err := dispatchCommand("read-object", []string{hash})
		if err != nil {
			t.Fatalf("read-object failed: %v", err)
		}
	})
	if !strings.Contains(out, "test content") {
		t.Errorf("expected content in output, got: %s", out)
	}
}

func TestCmdCommitWithRepo(t *testing.T) {
	dir := withTempRepo(t)

	blob := promptsheon.NewBlobObject("new data")
	bh, _ := promptsheon.WriteObject(blob)
	tree := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "file", Type: promptsheon.TypeBlob, Hash: bh},
	})
	th, _ := promptsheon.WriteObject(tree)

	_ = dir
	out := captureStdout(t, func() {
		err := dispatchCommand("commit", []string{th, "second commit"})
		if err != nil {
			t.Fatalf("commit failed: %v", err)
		}
	})
	if !strings.Contains(out, "second commit") {
		t.Errorf("expected commit message in output, got: %s", out)
	}
	if !strings.Contains(out, "]") {
		t.Errorf("expected commit hash in output, got: %s", out)
	}

	t.Run("commit with telemetry warning", func(t *testing.T) {
		blob2 := promptsheon.NewBlobObject("more data")
		bh2, _ := promptsheon.WriteObject(blob2)
		tree2 := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
			{Name: "file", Type: promptsheon.TypeBlob, Hash: bh2},
		})
		th2, _ := promptsheon.WriteObject(tree2)

		t.Setenv("PROMPTSHEON_TELEMETRY", "invalid json{{{")
		var stdout, stderr bytes.Buffer
		oldStdout, oldStderr := os.Stdout, os.Stderr
		rOut, wOut, _ := os.Pipe()
		rErr, wErr, _ := os.Pipe()
		os.Stdout = wOut
		os.Stderr = wErr
		err := dispatchCommand("commit", []string{th2, "third"})
		_ = wOut.Close()
		_ = wErr.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_, _ = stdout.ReadFrom(rOut)
		_, _ = stderr.ReadFrom(rErr)
		if err != nil {
			t.Fatalf("commit failed: %v", err)
		}
		if !strings.Contains(stderr.String(), "warning: telemetry ignored") {
			t.Errorf("expected telemetry warning on stderr, got stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "third") {
			t.Errorf("expected commit message in output, got: %s", stdout.String())
		}
	})
}

func TestCmdLog(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("log", nil)
		if err != nil {
			t.Fatalf("log failed: %v", err)
		}
	})
	if !strings.Contains(out, "Initial agent configuration") {
		t.Errorf("expected commit message in log, got: %s", out)
	}
	if !strings.Contains(out, "Author:") {
		t.Errorf("expected Author in log, got: %s", out)
	}
	if !strings.Contains(out, "accuracy") {
		t.Errorf("expected telemetry in log, got: %s", out)
	}
}

func TestCmdCheckout(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("checkout", []string{"main"})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}
	})
	if !strings.Contains(out, "switched to branch") {
		t.Errorf("expected switch message, got: %s", out)
	}
}

func TestCmdBranchWithRepo(t *testing.T) {
	_ = withTempRepo(t)

	t.Run("list branches", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("branch", nil)
			if err != nil {
				t.Fatalf("branch list failed: %v", err)
			}
		})
		if !strings.Contains(out, "main") {
			t.Errorf("expected main branch, got: %s", out)
		}
	})

	t.Run("create branch", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("branch", []string{"feature"})
			if err != nil {
				t.Fatalf("branch create failed: %v", err)
			}
		})
		if !strings.Contains(out, "created branch") {
			t.Errorf("expected created branch message, got: %s", out)
		}
	})
}

func TestCmdDeleteBranchWithRepo(t *testing.T) {
	_ = withTempRepo(t)

	_ = promptsheon.CreateBranch("feature", "")

	out := captureStdout(t, func() {
		err := dispatchCommand("delete-branch", []string{"feature"})
		if err != nil {
			t.Fatalf("delete-branch failed: %v", err)
		}
	})
	if !strings.Contains(out, "deleted branch") {
		t.Errorf("expected deleted branch message, got: %s", out)
	}
}

func TestCmdDiffWithRepo(t *testing.T) {
	_ = withTempRepo(t)

	hash1, _ := promptsheon.GetCurrentCommitHash()
	_ = promptsheon.CreateBranch("other", "")
	_ = promptsheon.Checkout("other")

	blob := promptsheon.NewBlobObject("different content")
	bh, _ := promptsheon.WriteObject(blob)
	tree := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "file", Type: promptsheon.TypeBlob, Hash: bh},
	})
	th, _ := promptsheon.WriteObject(tree)
	_, _ = promptsheon.Commit(th, nil, "engineer", "other commit", nil)
	hash2, _ := promptsheon.GetCurrentCommitHash()

	_ = promptsheon.Checkout("main")

	out := captureStdout(t, func() {
		err := dispatchCommand("diff", []string{hash1, hash2})
		if err != nil {
			t.Fatalf("diff failed: %v", err)
		}
	})
	if !strings.Contains(out, "Diff") {
		t.Errorf("expected Diff in output, got: %s", out)
	}
}

func TestCmdStatus(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("status", nil)
		if err != nil {
			t.Fatalf("status failed: %v", err)
		}
	})
	if !strings.Contains(out, "on branch main") {
		t.Errorf("expected 'on branch main', got: %s", out)
	}
	if !strings.Contains(out, "branches:") {
		t.Errorf("expected branches count, got: %s", out)
	}
	if !strings.Contains(out, "objects:") {
		t.Errorf("expected objects count, got: %s", out)
	}
}

func TestCmdShowBlob(t *testing.T) {
	_ = withTempRepo(t)

	obj := promptsheon.NewBlobObject("display content")
	hash, _ := promptsheon.WriteObject(obj)

	out := captureStdout(t, func() {
		err := dispatchCommand("show", []string{hash})
		if err != nil {
			t.Fatalf("show failed: %v", err)
		}
	})
	if !strings.Contains(out, "blob") {
		t.Errorf("expected blob in output, got: %s", out)
	}
	if !strings.Contains(out, "display content") {
		t.Errorf("expected content in output, got: %s", out)
	}
}

func TestCmdShowCommit(t *testing.T) {
	_ = withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()

	out := captureStdout(t, func() {
		err := dispatchCommand("show", []string{hash})
		if err != nil {
			t.Fatalf("show failed: %v", err)
		}
	})
	if !strings.Contains(out, "commit") {
		t.Errorf("expected commit in output, got: %s", out)
	}
	if !strings.Contains(out, "Initial agent configuration") {
		t.Errorf("expected commit message, got: %s", out)
	}
	if !strings.Contains(out, "Tree contents") {
		t.Errorf("expected tree contents, got: %s", out)
	}
	if !strings.Contains(out, "system-prompt") {
		t.Errorf("expected tree entry, got: %s", out)
	}
	if !strings.Contains(out, "accuracy") {
		t.Errorf("expected telemetry in output, got: %s", out)
	}
}

func TestCmdLsTree(t *testing.T) {
	_ = withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()
	commitObj, _ := promptsheon.ReadObject(hash)

	out := captureStdout(t, func() {
		err := dispatchCommand("ls-tree", []string{commitObj.TreeHash})
		if err != nil {
			t.Fatalf("ls-tree failed: %v", err)
		}
	})
	if !strings.Contains(out, "system-prompt") {
		t.Errorf("expected tree entries, got: %s", out)
	}
	if !strings.Contains(out, "tool-spec") {
		t.Errorf("expected tool-spec, got: %s", out)
	}
}

func TestCmdCatFile(t *testing.T) {
	_ = withTempRepo(t)

	obj := promptsheon.NewBlobObject("cat file content")
	hash, err := promptsheon.WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}

	out := captureStdout(t, func() {
		err := dispatchCommand("cat-file", []string{hash})
		if err != nil {
			t.Fatalf("cat-file failed: %v", err)
		}
	})
	if !strings.Contains(out, "cat file content") {
		t.Errorf("expected content, got: %s", out)
	}
}

func TestCmdGraph(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("graph", nil)
		if err != nil {
			t.Fatalf("graph failed: %v", err)
		}
	})
	if !strings.Contains(out, "o ") {
		t.Errorf("expected 'o ' in graph output, got: %s", out)
	}
	if !strings.Contains(out, "Initial agent") {
		t.Errorf("expected commit message in graph, got: %s", out)
	}
}

func TestCmdStats(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("stats", nil)
		if err != nil {
			t.Fatalf("stats failed: %v", err)
		}
	})
	if !strings.Contains(out, "blobs:") {
		t.Errorf("expected blobs in stats, got: %s", out)
	}
	if !strings.Contains(out, "refs:") {
		t.Errorf("expected refs in stats, got: %s", out)
	}
	if !strings.Contains(out, "storage:") {
		t.Errorf("expected storage in stats, got: %s", out)
	}
}

func TestCmdVerify(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("verify", nil)
		if err != nil {
			t.Fatalf("verify failed: %v", err)
		}
	})
	if !strings.Contains(out, "verified:") {
		t.Errorf("expected verified in output, got: %s", out)
	}
	if !strings.Contains(out, "repository is healthy") {
		t.Errorf("expected healthy, got: %s", out)
	}
}

func TestCmdRunArgumentParsing(t *testing.T) {
	t.Run("run with some flags but no prompt", func(t *testing.T) {
		err := dispatchCommand("run", []string{"--provider", "openai"})
		if err == nil {
			t.Fatal("expected error for missing prompt")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("run with prompt positional after flags", func(t *testing.T) {
		err := dispatchCommand("run", []string{"--provider", "x", "--model", "y", "hello"})
		if err == nil {
			t.Fatal("expected error because provider x doesn't exist")
		}
		if strings.Contains(err.Error(), "usage") {
			t.Errorf("expected provider error, not usage: %v", err)
		}
	})

	t.Run("run with short flags", func(t *testing.T) {
		err := dispatchCommand("run", []string{"-p", "x", "-m", "y", "hello"})
		if err == nil {
			t.Fatal("expected error because provider x doesn't exist")
		}
	})
}

func TestCmdProviderList(t *testing.T) {
	out := captureStdout(t, func() {
		err := dispatchCommand("provider", []string{"list"})
		if err != nil {
			t.Fatalf("provider list failed: %v", err)
		}
	})
	if !strings.Contains(out, "Registered providers") {
		t.Errorf("expected provider list header, got: %s", out)
	}
}

func TestCmdProviderTestNoProvider(t *testing.T) {
	err := dispatchCommand("provider", []string{"test", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
	if !strings.Contains(err.Error(), "provider not available") {
		t.Errorf("expected provider not available, got: %v", err)
	}
}

func TestCmdWorkspaceWithServer(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_SERVER")
	defer func() { _ = os.Setenv("PROMPTSHEON_SERVER", orig) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/workspaces":
			_, _ = w.Write([]byte(`[{"id":"w1","name":"test-workspace"}]`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/workspaces":
			_, _ = w.Write([]byte(`{"id":"w1","name":"new-workspace"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/workspaces/w1":
			_, _ = w.Write([]byte(`{"id":"w1","name":"test-workspace"}`))
		case r.Method == "DELETE" && r.URL.Path == "/api/v1/workspaces/w1":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()
	_ = os.Setenv("PROMPTSHEON_SERVER", server.URL)

	t.Run("list", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("workspace", []string{"list"})
			if err != nil {
				t.Fatalf("workspace list failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-workspace") {
			t.Errorf("expected workspace in output, got: %s", out)
		}
	})

	t.Run("create missing name", func(t *testing.T) {
		err := dispatchCommand("workspace", []string{"create"})
		if err == nil {
			t.Fatal("expected error for missing name")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("create", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("workspace", []string{"create", "new-workspace"})
			if err != nil {
				t.Fatalf("workspace create failed: %v", err)
			}
		})
		if !strings.Contains(out, "new-workspace") {
			t.Errorf("expected created workspace in output, got: %s", out)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		err := dispatchCommand("workspace", []string{"get"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("get", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("workspace", []string{"get", "w1"})
			if err != nil {
				t.Fatalf("workspace get failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-workspace") {
			t.Errorf("expected workspace in output, got: %s", out)
		}
	})

	t.Run("delete missing id", func(t *testing.T) {
		err := dispatchCommand("workspace", []string{"delete"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("workspace", []string{"delete", "w1"})
			if err != nil {
				t.Fatalf("workspace delete failed: %v", err)
			}
		})
		if !strings.Contains(out, "deleted") {
			t.Errorf("expected 'deleted' in output, got: %s", out)
		}
	})
}

func TestCmdProjectWithServer(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_SERVER")
	defer func() { _ = os.Setenv("PROMPTSHEON_SERVER", orig) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/workspaces/w1/projects":
			_, _ = w.Write([]byte(`[{"id":"p1","name":"test-project"}]`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/workspaces/w1/projects":
			_, _ = w.Write([]byte(`{"id":"p1","name":"new-project"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"test-project"}`))
		case r.Method == "DELETE" && r.URL.Path == "/api/v1/projects/p1":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()
	_ = os.Setenv("PROMPTSHEON_SERVER", server.URL)

	t.Run("list missing id", func(t *testing.T) {
		err := dispatchCommand("project", []string{"list"})
		if err == nil {
			t.Fatal("expected error for missing workspace id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("project", []string{"list", "w1"})
			if err != nil {
				t.Fatalf("project list failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-project") {
			t.Errorf("expected project in output, got: %s", out)
		}
	})

	t.Run("create missing args", func(t *testing.T) {
		err := dispatchCommand("project", []string{"create"})
		if err == nil {
			t.Fatal("expected error for missing args")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("create", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("project", []string{"create", "w1", "new-project"})
			if err != nil {
				t.Fatalf("project create failed: %v", err)
			}
		})
		if !strings.Contains(out, "new-project") {
			t.Errorf("expected new project in output, got: %s", out)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		err := dispatchCommand("project", []string{"get"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("get", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("project", []string{"get", "p1"})
			if err != nil {
				t.Fatalf("project get failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-project") {
			t.Errorf("expected project in output, got: %s", out)
		}
	})

	t.Run("delete missing id", func(t *testing.T) {
		err := dispatchCommand("project", []string{"delete"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("project", []string{"delete", "p1"})
			if err != nil {
				t.Fatalf("project delete failed: %v", err)
			}
		})
		if !strings.Contains(out, "deleted") {
			t.Errorf("expected 'deleted' in output, got: %s", out)
		}
	})
}

func TestCmdCapabilityWithServer(t *testing.T) {
	orig := os.Getenv("PROMPTSHEON_SERVER")
	defer func() { _ = os.Setenv("PROMPTSHEON_SERVER", orig) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/projects/p1/capabilities":
			_, _ = w.Write([]byte(`[{"id":"c1","name":"test-capability"}]`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/projects/p1/capabilities":
			_, _ = w.Write([]byte(`{"id":"c1","name":"new-capability"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/capabilities/c1":
			_, _ = w.Write([]byte(`{"id":"c1","name":"test-capability"}`))
		case r.Method == "DELETE" && r.URL.Path == "/api/v1/capabilities/c1":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()
	_ = os.Setenv("PROMPTSHEON_SERVER", server.URL)

	t.Run("list missing id", func(t *testing.T) {
		err := dispatchCommand("capability", []string{"list"})
		if err == nil {
			t.Fatal("expected error for missing project id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("capability", []string{"list", "p1"})
			if err != nil {
				t.Fatalf("capability list failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-capability") {
			t.Errorf("expected capability in output, got: %s", out)
		}
	})

	t.Run("create missing args", func(t *testing.T) {
		err := dispatchCommand("capability", []string{"create"})
		if err == nil {
			t.Fatal("expected error for missing args")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("create", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("capability", []string{"create", "p1", "new-capability"})
			if err != nil {
				t.Fatalf("capability create failed: %v", err)
			}
		})
		if !strings.Contains(out, "new-capability") {
			t.Errorf("expected new capability in output, got: %s", out)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		err := dispatchCommand("capability", []string{"get"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("get", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("capability", []string{"get", "c1"})
			if err != nil {
				t.Fatalf("capability get failed: %v", err)
			}
		})
		if !strings.Contains(out, "test-capability") {
			t.Errorf("expected capability in output, got: %s", out)
		}
	})

	t.Run("delete missing id", func(t *testing.T) {
		err := dispatchCommand("capability", []string{"delete"})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !errors.Is(err, errUsage) {
			t.Errorf("expected errUsage, got %T: %v", err, err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		out := captureStdout(t, func() {
			err := dispatchCommand("capability", []string{"delete", "c1"})
			if err != nil {
				t.Fatalf("capability delete failed: %v", err)
			}
		})
		if !strings.Contains(out, "deleted") {
			t.Errorf("expected 'deleted' in output, got: %s", out)
		}
	})
}

func TestDrawConnLinesDefaultFallback(t *testing.T) {
	var buf bytes.Buffer
	drawConnLines(&buf, 0, 0, 2, []int{0, 0, 0}, nil)
	want := "      \n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDrawConnLinesPrevColGTCol(t *testing.T) {
	var buf bytes.Buffer
	drawConnLines(&buf, 2, 0, 2, []int{0, 0, 0}, nil)
	want := "/   | \n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDrawMergeForkLinesSkipWhenNoColumnsBelowNoParentCols(t *testing.T) {
	var buf bytes.Buffer
	node := &promptsheon.GraphNode{Hash: "a1", Parents: []string{"b1", "c1"}}
	columns := map[string]int{"a1": 0, "b1": 0, "c1": 0}
	drawMergeForkLines(&buf, node, columns, 0, []int{0})
	if buf.Len() != 0 {
		t.Errorf("expected no output when parent cols equal node col, got %q", buf.String())
	}
}

func TestCmdShowTree(t *testing.T) {
	_ = withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()
	commitObj, _ := promptsheon.ReadObject(hash)
	treeHash := commitObj.TreeHash

	out := captureStdout(t, func() {
		err := dispatchCommand("show", []string{treeHash})
		if err != nil {
			t.Fatalf("show tree failed: %v", err)
		}
	})
	if !strings.Contains(out, "tree") {
		t.Errorf("expected tree header, got: %s", out)
	}
	if !strings.Contains(out, "system-prompt") {
		t.Errorf("expected tree entries, got: %s", out)
	}
}

func TestCmdShowEmptyObject(t *testing.T) {
	_ = withTempRepo(t)

	emptyObj := promptsheon.NewBlobObject("")
	hash, err := promptsheon.WriteObject(emptyObj)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}

	out := captureStdout(t, func() {
		err := dispatchCommand("show", []string{hash})
		if err != nil {
			t.Fatalf("show failed: %v", err)
		}
	})
	if !strings.Contains(out, "empty/unknown") {
		t.Errorf("expected empty/unknown message, got: %s", out)
	}
}

func TestCmdLsTreeNotATree(t *testing.T) {
	_ = withTempRepo(t)

	obj := promptsheon.NewBlobObject("not a tree")
	hash, _ := promptsheon.WriteObject(obj)

	err := dispatchCommand("ls-tree", []string{hash})
	if err == nil {
		t.Fatal("expected error for non-tree object")
	}
	if !strings.Contains(err.Error(), "is not a tree") {
		t.Errorf("expected 'is not a tree' error, got: %v", err)
	}
}

func TestCmdCatFileNotABlob(t *testing.T) {
	_ = withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()

	err := dispatchCommand("cat-file", []string{hash})
	if err == nil {
		t.Fatal("expected error for non-blob object")
	}
	if !strings.Contains(err.Error(), "is not a blob") {
		t.Errorf("expected 'is not a blob' error, got: %v", err)
	}
}

func TestCmdCatFileMissingNewline(t *testing.T) {
	_ = withTempRepo(t)

	obj := promptsheon.NewBlobObject("no newline at end")
	hash, err := promptsheon.WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}

	out := captureStdout(t, func() {
		err := dispatchCommand("cat-file", []string{hash})
		if err != nil {
			t.Fatalf("cat-file failed: %v", err)
		}
	})
	if !strings.Contains(out, "no newline at end") {
		t.Errorf("expected content, got: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline, got: %q", out)
	}
}

func TestCmdRunArgParsing(t *testing.T) {
	t.Run("positional only prompt", func(t *testing.T) {
		err := dispatchCommand("run", []string{"just a prompt"})
		if err == nil {
			t.Fatal("expected error since no provider")
		}
		if strings.Contains(err.Error(), "usage") {
			t.Errorf("expected non-usage error, got: %v", err)
		}
	})

	t.Run("--prompt flag", func(t *testing.T) {
		err := dispatchCommand("run", []string{"--prompt", "hello"})
		if err == nil {
			t.Fatal("expected error since no provider")
		}
		if strings.Contains(err.Error(), "usage") {
			t.Errorf("expected non-usage error, got: %v", err)
		}
	})
}

func TestCmdProviderTestMissingName(t *testing.T) {
	err := dispatchCommand("provider", []string{"test"})
	if err == nil {
		t.Fatal("expected error for missing provider name")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("expected errUsage, got %T: %v", err, err)
	}
}

func TestCmdProviderListOutput(t *testing.T) {
	out := captureStdout(t, func() {
		err := dispatchCommand("provider", []string{"list"})
		if err != nil {
			t.Fatalf("provider list: %v", err)
		}
	})
	if !strings.Contains(out, "Registered providers:") {
		t.Errorf("expected provider list, got: %s", out)
	}
}

func TestCmdGraphNoCommits(t *testing.T) {
	dir, err := os.MkdirTemp("", "promptsheon-empty-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	origWd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origWd) }()

	_ = promptsheon.Init()

	out := captureStdout(t, func() {
		err := dispatchCommand("graph", nil)
		if err != nil {
			t.Fatalf("graph: %v", err)
		}
	})
	if !strings.Contains(out, "no commits") {
		t.Errorf("expected 'no commits', got: %s", out)
	}
}

func TestCmdLogNoCommits(t *testing.T) {
	dir, err := os.MkdirTemp("", "promptsheon-empty-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	origWd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origWd) }()

	_ = promptsheon.Init()

	out := captureStdout(t, func() {
		err := dispatchCommand("log", nil)
		if err != nil {
			t.Fatalf("log: %v", err)
		}
	})
	if !strings.Contains(out, "no commits") {
		t.Errorf("expected 'no commits', got: %s", out)
	}
}

func TestCmdLogWithCount(t *testing.T) {
	_ = withTempRepo(t)
	out := captureStdout(t, func() {
		err := dispatchCommand("log", []string{"10"})
		if err != nil {
			t.Fatalf("log failed: %v", err)
		}
	})
	if !strings.Contains(out, "Initial agent configuration") {
		t.Errorf("expected commit message, got: %s", out)
	}
}

func TestCmdCheckoutDetached(t *testing.T) {
	dir := withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()

	_ = promptsheon.CreateBranch("other", "")
	_ = promptsheon.Checkout("other")

	blob := promptsheon.NewBlobObject("new")
	bh, _ := promptsheon.WriteObject(blob)
	tree := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "f", Type: promptsheon.TypeBlob, Hash: bh},
	})
	th, _ := promptsheon.WriteObject(tree)
	_, _ = promptsheon.Commit(th, nil, "e", "msg", nil)

	_ = dir
	out := captureStdout(t, func() {
		err := dispatchCommand("checkout", []string{hash})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}
	})
	if !strings.Contains(out, "HEAD is now at") {
		t.Errorf("expected detached HEAD message, got: %s", out)
	}
}

func TestCmdBranchCreateWithHash(t *testing.T) {
	_ = withTempRepo(t)

	hash, _ := promptsheon.GetCurrentCommitHash()

	out := captureStdout(t, func() {
		err := dispatchCommand("branch", []string{"newbranch", hash})
		if err != nil {
			t.Fatalf("branch create failed: %v", err)
		}
	})
	if !strings.Contains(out, "created branch") {
		t.Errorf("expected created branch, got: %s", out)
	}
}

func TestCmdShowTreeNested(t *testing.T) {
	_ = withTempRepo(t)

	inner := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "nested.txt", Type: promptsheon.TypeBlob, Hash: "0000000000000000000000000000000000000000000000000000000000000000"},
	})
	innerHash, _ := promptsheon.WriteObject(inner)

	b1 := promptsheon.NewBlobObject("root content")
	bh, _ := promptsheon.WriteObject(b1)

	outer := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "inner", Type: promptsheon.TypeTree, Hash: innerHash},
		{Name: "root.txt", Type: promptsheon.TypeBlob, Hash: bh},
	})
	outerHash, _ := promptsheon.WriteObject(outer)

	out := captureStdout(t, func() {
		err := dispatchCommand("show", []string{outerHash})
		if err != nil {
			t.Fatalf("show tree: %v", err)
		}
	})
	if !strings.Contains(out, "inner") {
		t.Errorf("expected inner tree entry, got: %s", out)
	}
	if !strings.Contains(out, "root.txt") {
		t.Errorf("expected root file entry, got: %s", out)
	}
}

func TestCmdLsTreeRecursive(t *testing.T) {
	_ = withTempRepo(t)

	inner := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "nested.txt", Type: promptsheon.TypeBlob, Hash: "0000000000000000000000000000000000000000000000000000000000000000"},
	})
	innerHash, _ := promptsheon.WriteObject(inner)

	outer := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "inner", Type: promptsheon.TypeTree, Hash: innerHash},
	})
	outerHash, _ := promptsheon.WriteObject(outer)

	out := captureStdout(t, func() {
		err := dispatchCommand("ls-tree", []string{outerHash})
		if err != nil {
			t.Fatalf("ls-tree: %v", err)
		}
	})
	if !strings.Contains(out, "inner/nested.txt") {
		t.Errorf("expected nested path, got: %s", out)
	}
}

func TestFormatBytesEdgeCases(t *testing.T) {
	if got := formatBytes(0); got != "0 B" {
		t.Errorf("expected 0 B, got %q", got)
	}
	if got := formatBytes(1); got != "1 B" {
		t.Errorf("expected 1 B, got %q", got)
	}
	if got := formatBytes(1024); got != "1.0 KiB" {
		t.Errorf("expected 1.0 KiB, got %q", got)
	}
	if got := formatBytes(1024 * 1024); got != "1.0 MiB" {
		t.Errorf("expected 1.0 MiB, got %q", got)
	}
	if got := formatBytes(1024*1024 + 512*1024); got != "1.5 MiB" {
		t.Errorf("expected 1.5 MiB, got %q", got)
	}
}

func TestAssignGraphColumnsNoMatchingRef(t *testing.T) {
	nodes := []*promptsheon.GraphNode{
		{Hash: "a1", Parents: []string{}, IsHEAD: true},
	}
	refs := []*promptsheon.RefDetail{
		{Name: "main", Hash: "b1"},
	}
	cols := assignGraphColumns(nodes, refs, "main")
	if cols["a1"] != 0 {
		t.Errorf("expected a1 in col 0, got %d", cols["a1"])
	}
}

func TestDrawMergeForkLinesMultipleParentsBelow(t *testing.T) {
	var buf bytes.Buffer
	node := &promptsheon.GraphNode{Hash: "c1", Parents: []string{"a1", "b1", "d1"}}
	columns := map[string]int{"c1": 1, "a1": 0, "b1": 1, "d1": 2}
	drawMergeForkLines(&buf, node, columns, 2, []int{0, 0, 0})
	want := "|   / \n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDrawConnLinesPrevColGTColWithColRemaining(t *testing.T) {
	var buf bytes.Buffer
	drawConnLines(&buf, 2, 1, 2, []int{1, 0, 0}, nil)
	want := "| / | \n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDrawConnLinesAllColsBelow(t *testing.T) {
	var buf bytes.Buffer
	drawConnLines(&buf, 0, 1, 2, []int{0, 0, 0}, nil)
	want := "| \\   \n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDrawMergeForkLinesNoParentColsDifferent(t *testing.T) {
	var buf bytes.Buffer
	node := &promptsheon.GraphNode{Hash: "c1", Parents: []string{"a1", "b1"}}
	columns := map[string]int{"c1": 0, "a1": 0, "b1": 0}
	drawMergeForkLines(&buf, node, columns, 0, []int{0})
	if buf.Len() != 0 {
		t.Errorf("expected no output when all parent cols same as node col, got %q", buf.String())
	}
}

func TestRenderGraphBranchLabels(t *testing.T) {
	nodes := []*promptsheon.GraphNode{
		{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Parents: []string{}, Branches: []string{"main", "backup"}, IsHEAD: true, Message: "multi"},
	}
	columns := map[string]int{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 0}
	out := renderGraph(nodes, columns, 0)
	if !strings.Contains(out, "HEAD -> main") {
		t.Errorf("expected HEAD -> main label, got: %s", out)
	}
	if !strings.Contains(out, "backup") {
		t.Errorf("expected backup label, got: %s", out)
	}
}

func TestRenderGraphBranchLabelsNonHEAD(t *testing.T) {
	nodes := []*promptsheon.GraphNode{
		{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Parents: []string{}, Branches: []string{"feature"}, Message: "feat"},
	}
	columns := map[string]int{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 0}
	out := renderGraph(nodes, columns, 0)
	if !strings.Contains(out, "feature") {
		t.Errorf("expected feature label, got: %s", out)
	}
	if strings.Contains(out, "HEAD") {
		t.Errorf("unexpected HEAD label, got: %s", out)
	}
}

func TestRenderGraphNoMessage(t *testing.T) {
	nodes := []*promptsheon.GraphNode{
		{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Parents: []string{}},
	}
	columns := map[string]int{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 0}
	out := renderGraph(nodes, columns, 0)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	line := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(line, "o ") {
		t.Errorf("expected 'o ' prefix, got: %s", line)
	}
}
