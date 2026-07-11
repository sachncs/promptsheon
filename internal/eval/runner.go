// Package eval provides evaluation runners for capability versions.
package eval

import (
	"github.com/sachncs/promptsheon/internal/llm"
)

// Runner executes evaluation runs for capability versions. F-05
// forward-only: the legacy RunVersion / buildVersionPrompt
// analyzers that consumed the deleted bundle types (Prompt,
// ModelPolicy, etc.) are gone. The Runner type stays because
// production wiring will use it for v2+ RunVersion signatures
// (signature lands with the v0.1.0 follow-on).
type Runner struct {
	provider llm.Provider
}

// NewRunner creates a new Runner with the given provider.
func NewRunner(provider llm.Provider) *Runner {
	return &Runner{provider: provider}
}
