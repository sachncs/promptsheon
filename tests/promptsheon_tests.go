// Package tests — test bodies for the single-runner architecture.
//
// Each test function is added to the AllTests slice; the
// promptsheon package's single _test.go entry point (TestPromptsheon)
// dispatches into tests.RunAll which iterates the registry.
//
// Convention:
//   - Function name: Run<Subject> (exported; the test runner calls them)
//   - File name:    <subject>.go (no _test.go suffix; the test runner
//                    picks them up by name, not by file discovery)
//   - Package:      tests
//
// To add a new test: drop a `func RunXxx(t *testing.T)` into
// tests/<file>.go and append it to AllTests below.
package tests

import "testing"

// AllTests is the registry of every test in this package. Append
// new test functions here; the runner picks them up automatically.
var AllTests = []func(t *testing.T){
	RunSmoke,
}

// RunAll iterates AllTests and invokes each function. The standard
// testing.T framework accumulates per-test failures and reports
// them at the end of the run.
func RunAll(t *testing.T) {
	for _, fn := range AllTests {
		fn(t)
	}
}

// RunSmoke is a placeholder test that verifies the runner wiring
// is correct. It runs as part of `go test ./promptsheon/` and
// succeeds with no side effects. As more tests are moved into
// the tests/ package, their Run<Subject> functions are appended
// to AllTests.
func RunSmoke(t *testing.T) {
	// Intentionally empty; the runner exercising RunAll is
	// itself the verification.
	_ = t
}
