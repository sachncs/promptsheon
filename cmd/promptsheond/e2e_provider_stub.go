//go:build !e2e

// e2e_provider_stub.go — default-release counterpart to
// e2e_provider.go. When the build tag is absent, this stub is
// compiled instead and registerE2EProvider is a no-op.
//
// The return type uses `any` (not *llm.InMemoryProvider) because
// llm/inmemory_e2e.go is gated behind //go:build e2e and isn't
// part of the default build. Callers cast the result to the
// e2e-only concrete type via the //go:build e2e test file.
package main

import (
	"log/slog"

	"github.com/sachncs/promptsheon/promptsheon/llm"
)

// registerE2EProvider is a no-op in default builds. The matching
// e2e-tagged version installs the in-memory LLM stub.
//
// U1000 (unused): the function is only called from the e2e
// daemon (cmd/promptsheond/daemon.go inside the //go:build e2e
// block). staticcheck can't see across build tags and reports
// the stub as unused. The linter baseline accepts this single
// finding; if a future change calls registerE2EProvider from
// the default build, remove the stub file.
func registerE2EProvider(reg *llm.Registry, logger *slog.Logger) any { // #nosec U1000 -- used only in e2e builds; see comment above
	_ = reg
	_ = logger
	return nil
}
