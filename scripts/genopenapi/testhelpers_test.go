package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// packageDir returns the absolute path of the genopenapi
// package (the directory containing this test helper file).
// We use runtime.Caller to find this file at runtime, so the
// tests are independent of the cwd the user happened to run
// them from.
func packageDir() string {
	var file string
	_, file, _, _ = runtime.Caller(0)
	return filepath.Dir(file)
}

// buildTool compiles the genopenapi command into the given path.
// We rebuild on every test invocation so a stale binary never
// silently passes. The package directory is the directory
// containing the tool's source; we use it as the build root.
func buildTool(out string) error {
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = packageDir()
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runBinary executes a previously-built binary with the given
// args. Stdout/Stderr are inherited so the test sees the same
// output the user would see.
func runBinary(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// repoRoot walks up from the current directory until it finds
// go.mod. The tool expects to be run from the repo root, so
// the tests do the same.
func repoRoot(t interface{ Helper() }) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Fell off the tree. Fall back to the
			// original cwd so the caller at least gets
			// a sensible error.
			return wd
		}
		dir = parent
	}
}
