// Package tests is a non-_test.go test container.
//
// The conventional Go test runner picks up files matching *_test.go
// in any package directory. This package uses files without the
// _test.go suffix so the user can keep all test bodies in one
// tree (per the compliance plan, Part F) without losing the
// option to run them via a single entry point.
//
// The promptsheon package contains a single real _test.go file
// (promptsheon/promptsheon_test.go) that calls tests.RunAll to
// dispatch into the Run* functions defined here.
package tests

import "testing"

// AllTests is the registry of every test function in this
// package. The promptsheon package's single test entry point
// iterates this slice to run all tests in one go test invocation.
var AllTests = []func(t *testing.T){}

// RunAll iterates AllTests and invokes each one. Tests that
// fail do not stop the iteration; the standard testing.T
// framework accumulates the failures and reports them at the end.
func RunAll(t *testing.T) {
	for _, fn := range AllTests {
		fn(t)
	}
}
