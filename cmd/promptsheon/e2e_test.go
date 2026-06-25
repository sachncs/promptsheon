package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"promptsheon/internal/promptsheon"
)

var cliBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "promptsheon-cli-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdtemp: %v\n", err)
		os.Exit(1)
	}
	cliBinary = filepath.Join(dir, "promptsheon")
	cmd := exec.Command("go", "build", "-o", cliBinary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func setupE2ERepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "promptsheon-e2e-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(oldWd) })

	if err := promptsheon.Init(); err != nil {
		t.Fatalf("Init(): %v", err)
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

	b4 := promptsheon.NewBlobObject("optimized hyperparams: temp=0.3")
	h4, _ := promptsheon.WriteObject(b4)
	tree2 := promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "system-prompt", Type: promptsheon.TypeBlob, Hash: h1},
		{Name: "tool-spec", Type: promptsheon.TypeBlob, Hash: h2},
		{Name: "hyperparams", Type: promptsheon.TypeBlob, Hash: h4},
	})
	treeHash2, err := promptsheon.WriteObject(tree2)
	if err != nil {
		t.Fatalf("WriteObject(tree2): %v", err)
	}

	if err := promptsheon.CreateBranch("experimental", ""); err != nil {
		t.Fatalf("CreateBranch(): %v", err)
	}
	if err := promptsheon.Checkout("experimental"); err != nil {
		t.Fatalf("Checkout(): %v", err)
	}
	tel2 := map[string]any{"accuracy": 0.98, "latency_ms": 350}
	_, err = promptsheon.Commit(treeHash2, nil, "engineer", "Optimize hyperparams", tel2)
	if err != nil {
		t.Fatalf("Commit() on experimental: %v", err)
	}

	if err := promptsheon.Checkout("main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	return dir
}

func runCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(cliBinary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("promptsheon %v failed: %v\noutput:\n%s", args, err, out)
	}
	return string(out)
}

func TestE2EStatus(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "status")
	if !strings.Contains(out, "on branch main") {
		t.Fatalf("expected 'on branch main', got:\n%s", out)
	}
	if !strings.Contains(out, "branches: 2") {
		t.Fatalf("expected 'branches: 2', got:\n%s", out)
	}
}

func TestE2EStats(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "stats")
	if !strings.Contains(out, "blobs:") {
		t.Fatalf("expected stats output, got:\n%s", out)
	}
	if !strings.Contains(out, "refs:") {
		t.Fatalf("expected refs in stats, got:\n%s", out)
	}
}

func TestE2EVerify(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "verify")
	if !strings.Contains(out, "repository is healthy") {
		t.Fatalf("expected healthy repo, got:\n%s", out)
	}
}

func TestE2EGraph(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "graph")
	if !strings.Contains(out, "Initial agent") {
		t.Fatalf("expected commits in graph output, got:\n%s", out)
	}
	if !strings.Contains(out, "experimental") {
		t.Fatalf("expected branch labels in graph, got:\n%s", out)
	}
}

func TestE2EShowBlob(t *testing.T) {
	dir := setupE2ERepo(t)

	obj := promptsheon.NewBlobObject("system prompt: You are a helpful assistant")
	hash, err := promptsheon.ObjectHash(obj)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}

	out := runCLI(t, dir, "show", hash)
	if !strings.Contains(out, "blob") {
		t.Fatalf("expected 'blob' in show output, got:\n%s", out)
	}
	if !strings.Contains(out, "system prompt") {
		t.Fatalf("expected content in show output, got:\n%s", out)
	}
}

func TestE2ECatFile(t *testing.T) {
	dir := setupE2ERepo(t)

	obj := promptsheon.NewBlobObject("test content for cat-file")
	hash, err := promptsheon.WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject(): %v", err)
	}

	out := runCLI(t, dir, "cat-file", hash)
	if !strings.Contains(out, "test content for cat-file") {
		t.Fatalf("expected content output, got:\n%s", out)
	}
}

func TestE2ELsTree(t *testing.T) {
	dir := setupE2ERepo(t)

	hash, err := promptsheon.GetCurrentCommitHash()
	if err != nil {
		t.Fatalf("GetCurrentCommitHash(): %v", err)
	}
	commitObj, err := promptsheon.ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject(): %v", err)
	}

	out := runCLI(t, dir, "ls-tree", commitObj.TreeHash)
	if !strings.Contains(out, "system-prompt") {
		t.Fatalf("expected tree entries, got:\n%s", out)
	}
	if !strings.Contains(out, "tool-spec") {
		t.Fatalf("expected tree entries, got:\n%s", out)
	}
}

func TestE2EShowCommit(t *testing.T) {
	dir := setupE2ERepo(t)

	hash, err := promptsheon.GetCurrentCommitHash()
	if err != nil {
		t.Fatalf("GetCurrentCommitHash(): %v", err)
	}

	out := runCLI(t, dir, "show", hash)
	if !strings.Contains(out, "commit") {
		t.Fatalf("expected 'commit' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Initial agent") {
		t.Fatalf("expected commit message, got:\n%s", out)
	}
	if !strings.Contains(out, "system-prompt") {
		t.Fatalf("expected tree contents, got:\n%s", out)
	}
	if !strings.Contains(out, "accuracy") {
		t.Fatalf("expected telemetry in output, got:\n%s", out)
	}
}

func TestE2EBranchList(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "branch")
	if !strings.Contains(out, "main") {
		t.Fatalf("expected main branch, got:\n%s", out)
	}
	if !strings.Contains(out, "experimental") {
		t.Fatalf("expected experimental branch, got:\n%s", out)
	}
}

func TestE2ELog(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "log", "5")
	if !strings.Contains(out, "Initial agent") {
		t.Fatalf("expected commit in log, got:\n%s", out)
	}
}

func TestE2EDiff(t *testing.T) {
	dir := setupE2ERepo(t)

	hash1, _ := promptsheon.GetCurrentCommitHash()
	_ = promptsheon.Checkout("experimental")
	hash2, _ := promptsheon.GetCurrentCommitHash()
	_ = promptsheon.Checkout("main")

	out := runCLI(t, dir, "diff", hash1, hash2)
	if !strings.Contains(out, "Diff") {
		t.Fatalf("expected diff output, got:\n%s", out)
	}
}

func TestE2EHashObject(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "hash-object", "test data")
	out = strings.TrimSpace(out)
	if len(out) != 64 {
		t.Fatalf("expected 64-char hash, got %q (len=%d)", out, len(out))
	}
}

func TestE2EWriteReadObject(t *testing.T) {
	dir := setupE2ERepo(t)
	hashOut := runCLI(t, dir, "write-object", "round trip test")
	hash := strings.TrimSpace(hashOut)
	if len(hash) != 64 {
		t.Fatalf("expected 64-char hash, got %q", hash)
	}

	readOut := runCLI(t, dir, "read-object", hash)
	if !strings.Contains(readOut, "round trip test") {
		t.Fatalf("expected content in read output, got:\n%s", readOut)
	}
}

func TestE2EHelp(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "help")
	if !strings.Contains(out, "status") {
		t.Fatalf("expected status in help, got:\n%s", out)
	}
	if !strings.Contains(out, "graph") {
		t.Fatalf("expected graph in help, got:\n%s", out)
	}
	if !strings.Contains(out, "show") {
		t.Fatalf("expected show in help, got:\n%s", out)
	}
	if !strings.Contains(out, "ls-tree") {
		t.Fatalf("expected ls-tree in help, got:\n%s", out)
	}
	if !strings.Contains(out, "cat-file") {
		t.Fatalf("expected cat-file in help, got:\n%s", out)
	}
	if !strings.Contains(out, "stats") {
		t.Fatalf("expected stats in help, got:\n%s", out)
	}
	if !strings.Contains(out, "verify") {
		t.Fatalf("expected verify in help, got:\n%s", out)
	}
}

func TestE2EInitError(t *testing.T) {
	dir := setupE2ERepo(t)
	cmd := exec.Command(cliBinary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for double init")
	}
	if !strings.Contains(string(out), "already initialized") {
		t.Fatalf("expected 'already initialized', got:\n%s", out)
	}
}

func TestE2EGraphOutputFormat(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "graph")

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected graph output")
	}

	// Lines may start with "o ", "| o ", etc. depending on column layout.
	// Check that at least one line contains a commit marker.
	hasCommitMarker := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "o ") {
			hasCommitMarker = true
			break
		}
	}
	if !hasCommitMarker {
		t.Fatalf("expected at least one line with 'o ', got:\n%s", out)
	}

	// Check that the output contains abbreviated hashes (12 chars).
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		rest := strings.TrimPrefix(trimmed, "o ")
		fields := strings.Fields(rest)
		if len(fields) > 0 && len(fields[0]) == 12 {
			return // found a hash
		}
	}
	t.Fatalf("expected at least one line with 12-char hash, got:\n%s", out)
}

func TestE2EUninitializedCommand(t *testing.T) {
	dir, _ := os.MkdirTemp("", "promptsheon-not-init-*")
	defer os.RemoveAll(dir)

	cmds := []string{"status", "stats", "verify", "graph"}
	for _, cmd := range cmds {
		c := exec.Command(cliBinary, cmd)
		c.Dir = dir
		out, _ := c.CombinedOutput()
		if !strings.Contains(string(out), "not a promptsheon") {
			t.Fatalf("command %q should give uninitialized error, got:\n%s", cmd, out)
		}
	}
}

func TestE2EHelpNoDemo(t *testing.T) {
	dir := setupE2ERepo(t)
	out := runCLI(t, dir, "help")
	if strings.Contains(out, "demo") {
		t.Fatal("demo command should not appear in help")
	}
}

func TestE2EBadTelemetryWarns(t *testing.T) {
	dir := setupE2ERepo(t)

	b := promptsheon.NewBlobObject("telemetry test")
	bh, _ := promptsheon.WriteObject(b)
	th, _ := promptsheon.WriteObject(promptsheon.NewTreeObject([]promptsheon.TreeEntry{
		{Name: "file", Type: promptsheon.TypeBlob, Hash: bh},
	}))

	hash, _ := promptsheon.GetCurrentCommitHash()
	promptsheon.WriteRef("main", hash)

	cmd := exec.Command(cliBinary, "commit", th, "telemetry test commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PROMPTSHEON_TELEMETRY=not valid json{{")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit should succeed despite bad telemetry: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "warning: telemetry ignored") {
		t.Fatalf("expected telemetry warning, got:\n%s", out)
	}
}
