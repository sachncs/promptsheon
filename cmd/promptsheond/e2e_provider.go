//go:build e2e

// e2e_provider.go — wires the in-memory LLM provider into the
// daemon's LLM registry when built with -tags=e2e.
//
// The default release build does NOT include this file (it carries
// //go:build e2e so it's compiled only when -tags=e2e is set).
// The matching stub file (e2e_provider_stub.go) is the //go:build !e2e
// counterpart so `go build` works without the tag and stays a no-op.
package main

import (
	"log/slog"

	"github.com/sachncs/promptsheon/promptsheon/llm"
)

// registerE2EProvider wires the in-memory LLM stub into the given
// registry. Tests call this from a setup helper before starting
// the daemon, then assert on llm.InMemoryProvider.Calls() to
// verify the lifecycle reached the LLM invoke step.
func registerE2EProvider(reg *llm.Registry, logger *slog.Logger) any {
	p := llm.RegisterInMemoryProvider(reg)
	if logger != nil {
		logger.Info("e2e: in-memory LLM provider registered",
			"calls_so_far", p.Calls())
	}
	return p
}
