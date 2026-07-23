package harness_test

import (
	"github.com/sachncs/promptsheon/internal/testutil/harnessrepo"
)

// newRepo returns a fresh in-memory harness.Repository for
// tests. The previous implementation lived entirely in this
// package as memRepo; TEST-3 consolidated the fixture into
// internal/testutil/harnessrepo so release tests can share
// the same boilerplate. The local helper remains so the
// existing test call sites (newRepo(t)) keep compiling
// without changes.
//
// The returned value satisfies harness.Repository.
func newRepo(t testingTB) *harnessrepo.MemRepo {
	t.Helper()
	return harnessrepo.New()
}

type testingTB interface {
	Cleanup(func())
	Helper()
	Errorf(format string, args ...any)
	FailNow()
}
