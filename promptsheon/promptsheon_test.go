// Package promptsheon — single test entry point.
//
// The conventional Go test runner picks up this _test.go file
// (it ends in _test.go) and runs TestPromptsheon. That function
// calls tests.RunAll, which iterates the AllTests slice in the
// tests/ package.
//
// To add a new test: write a `func RunXxx(t *testing.T)` in
// tests/<file>.go and append it to AllTests. The test function
// does NOT need to end in _test.go suffix to be discoverable —
// the test runner picks it up by name.
package promptsheon_test

import (
	"testing"

	"github.com/sachncs/promptsheon/tests"
)

// TestPromptsheon is the single test entry point for the entire
// repository. It dispatches to the tests/ package, which holds
// every test body. This is the convention established by the
// compliance refactor (Part E): a single runner, no scattered
// _test.go files.
func TestPromptsheon(t *testing.T) {
	tests.RunAll(t)
}
