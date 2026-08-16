package evolve

import (
	"context"
	"encoding/json"

	"github.com/sachncs/promptsheon/errf"
)

// LLMRevisionStrategy asks a revision LLM for a new
// prompt. The wire format is JSON: the LLM's "user"
// message is the serialised ReviseRequest; the LLM
// returns the new prompt as plain text in its response.
type LLMRevisionStrategy struct {
	Invoke LLMInvokeFn
}

// NewLLMRevisionStrategy wires the strategy to the
// supplied invoke function. Tests pass a fake; production
// wires it to the daemon's llm.Registry.
func NewLLMRevisionStrategy(invoke LLMInvokeFn) *LLMRevisionStrategy {
	return &LLMRevisionStrategy{Invoke: invoke}
}

// Revise builds the wire payload, calls the LLM, and
// returns the new prompt text. The LLM is expected to
// return plain text per DefaultRevisionLLMSystem. Empty
// responses are rejected.
func (s *LLMRevisionStrategy) Revise(ctx context.Context, req ReviseRequest) (*ReviseResponse, error) {
	if s.Invoke == nil {
		return nil, errf.Errorf("selfevolve: revision LLM not wired")
	}
	if req.CurrentPrompt == "" {
		return nil, errf.Errorf("selfevolve: empty current prompt")
	}
	payload, err := json.Marshal(struct {
		CurrentPrompt   string        `json:"current_prompt"`
		CurrentHash     string        `json:"current_hash"`
		ModelPolicyHash string        `json:"model_policy_hash"`
		FailingCases    []FailingCase `json:"failing_cases"`
	}{
		CurrentPrompt:   req.CurrentPrompt,
		CurrentHash:     req.CurrentHash,
		ModelPolicyHash: req.ModelPolicyHash,
		FailingCases:    req.FailingCases,
	})
	if err != nil {
		return nil, errf.Errorf("selfevolve: marshal revise request: %w", err)
	}
	out, err := s.Invoke(ctx, LLMInvokeRequest{
		System: DefaultRevisionLLMSystem,
		User:   string(payload),
	})
	if err != nil {
		return nil, errf.Errorf("selfevolve: revision LLM call: %w", err)
	}
	if out == "" {
		return nil, errf.Errorf("selfevolve: revision LLM returned empty prompt")
	}
	return &ReviseResponse{NewPrompt: out}, nil
}
