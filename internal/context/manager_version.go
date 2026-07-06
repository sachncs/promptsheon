package context

import (
	"fmt"

	"github.com/sachn-cs/promptsheon/internal/capability"
)

// AssembleFromContract assembles context from a ContextContract.
//
// This is the capability-centric equivalent of Assemble(*models.Context).
// It validates the contract's requirements and produces an AssembledContext
// that the runtime can use for execution.
func (m *Manager) AssembleFromContract(contract *capability.ContextContract) (*AssembledContext, error) {
	if contract == nil {
		return nil, fmt.Errorf("context contract is required")
	}

	assembled := &AssembledContext{
		SystemMessage: "",
		Messages:      nil,
		TokenCount:    0,
		Truncated:     false,
		Strategy:      contract.CompressionStrategy,
	}

	// Validate required context
	for _, ref := range contract.RequiredContext {
		if ref.Key == "" {
			return nil, fmt.Errorf("required context reference has empty key")
		}
	}

	// Apply maximum size constraint
	if contract.MaximumSize > 0 {
		assembled.TokenCount = contract.MaximumSize
	}

	return assembled, nil
}
