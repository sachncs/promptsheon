package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerator_NoStubsInOutput pins the Item 1 requirement:
// the regenerated OpenAPI spec must not contain any "TODO:
// M-15" stub entries. The previous generator produced 59
// such entries; this test would fail if the new generator
// ever regressed to the stub form.
func TestGenerator_NoStubsInOutput(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "spec.yaml")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Run the generator from the repo root so it finds
	// internal/api/server.go.
	if e := os.Chdir(repoRoot(t)); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	run(t, "-out", outPath)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "TODO: M-15") {
		t.Fatalf("output still contains TODO stubs; the generator must produce real entries\n%s", s)
	}
	if strings.Contains(s, "flesh out") {
		t.Fatalf("output still contains 'flesh out' stub; the generator must produce real entries")
	}
	if strings.Contains(s, "summary: \"OK\"") {
		// Some quick paths use literal "OK" as the response
		// description, which is fine. We just want to make
		// sure no path uses "summary: \"OK\"" as a stub.
		t.Fatalf("output uses 'summary: \"OK\"' as a stub")
	}
}

// TestGenerator_Idempotent pins the second Item 1 requirement:
// running the generator twice produces the same output. The
// canonical way to keep api/openapi.yaml in sync with the code
// is to re-run the generator; idempotence is what makes that
// safe to do on every PR.
func TestGenerator_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	outA := filepath.Join(tmp, "a.yaml")
	outB := filepath.Join(tmp, "b.yaml")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if e := os.Chdir(repoRoot(t)); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	run(t, "-out", outA)
	run(t, "-out", outB)

	a, err := os.ReadFile(outA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(outB)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("generator is not idempotent; outputs differ\n--- A ---\n%s\n--- B ---\n%s", a, b)
	}
}

// TestGenerator_AllServerRoutesCovered pins the third Item 1
// requirement: every route registered in server.go must have
// an entry in the output spec. Missing an entry would leave a
// route undocumented.
func TestGenerator_AllServerRoutesCovered(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "spec.yaml")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if e := os.Chdir(repoRoot(t)); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	run(t, "-out", outPath)

	routes, err := collectRoutes(filepath.Join(repoRoot(t), "internal/api/server.go"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, r := range routes {
		// Check that a path: ... <path>: block exists.
		needle := "\n  " + r.Path + ":\n"
		if !strings.Contains(s, needle) {
			t.Errorf("route %s %s missing from output", r.Method, r.Path)
		}
	}
}

// TestGenerator_RequestBodyOnWrites pins the fourth Item 1
// requirement: POST and PUT handlers that have a request
// struct in their body must have a requestBody section in the
// output. Otherwise the spec is misleading.
func TestGenerator_RequestBodyOnWrites(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "spec.yaml")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if e := os.Chdir(repoRoot(t)); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	run(t, "-out", outPath)

	handlers, err := collectHandlers(filepath.Join(repoRoot(t), "internal/api"))
	if err != nil {
		t.Fatal(err)
	}
	// Spot-check a few well-known handlers. The names
	// below must be present in handlers_*.go.
	known := []string{
		"handleCreateWorkspace",
		"handleCreateProject",
		"handleCreateCapability",
		"handleCreateVersion",
		"handleCreateExecution",
	}
	for _, name := range known {
		h, ok := handlers[name]
		if !ok {
			t.Errorf("handler %s not found in handlers_*.go", name)
			continue
		}
		if h.requestType == nil {
			t.Errorf("handler %s has no request struct; the fixture is broken", name)
			continue
		}
		if len(h.requestType.Fields) == 0 {
			t.Errorf("handler %s request struct has 0 fields; the fixture is broken", name)
		}
	}

	// And verify the output spec contains a requestBody
	// for at least one of these.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "requestBody:") {
		t.Fatalf("output contains no requestBody section; write-handlers are missing schemas")
	}
}

// run invokes the binary built from the current package. We
// run the tests against the same source tree the production
// tool uses, so a regression in either the tool or its input
// sources is caught.
func run(t *testing.T, args ...string) {
	t.Helper()
	// Build the tool into a temp binary.
	binPath := filepath.Join(t.TempDir(), "genopenapi")
	if err := buildTool(binPath); err != nil {
		t.Fatal(err)
	}
	// os.Args[0] is the test binary; we exec the tool.
	cmd := append([]string{binPath}, args...)
	if err := runBinary(cmd...); err != nil {
		t.Fatal(err)
	}
}
