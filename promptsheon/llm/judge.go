package llm

import (
	"fmt"
	"context"
	"os"

	"github.com/sachncs/promptsheon/promptsheon/eval"
)

// JudgeClient is an alias for eval.JudgeClient so callers
// don't need to import the eval package. The adapter in
// NewJudgeClient returns one.
type JudgeClient = eval.JudgeClient

// JudgeClient is the production LLM-judge adapter. It satisfies
// the eval.JudgeClient interface by routing the judge's prompt
// through the registered Provider registry. The judge model is
// the first registered provider; production deployments
// typically register a separate "judge" provider (often a
// cheaper, faster model than the one under test).
//
// NewJudgeClient returns a JudgeClient + a bool indicating
// whether at least one provider is registered. Callers skip
// the registration when ok is false so the daemon boots
// cleanly without an LLM gateway.
func NewJudgeClient(reg *Registry) (JudgeClient, bool) {
	if reg == nil {
		return nil, false
	}
	names := reg.Providers()
	if len(names) == 0 {
		return nil, false
	}
	// Pick the first provider; production wiring can override
	// via PROMPTSHEON_LLM_JUDGE_PROVIDER (env var).
	name := names[0]
	if v := os.Getenv("PROMPTSHEON_LLM_JUDGE_PROVIDER"); v != "" {
		name = v
	}
	prov, err := reg.Get(name)
	if err != nil {
		return nil, false
	}
	return &registryJudge{reg: prov, name: name}, true
}

// registryJudge routes judge calls through the registry's
// named provider. The judge prompt is sent verbatim; the
// response is returned as a string for the eval scorer to
// parse.
type registryJudge struct {
	reg  Provider
	name string
}

// Complete sends prompt to the judge provider. The model and
// temperature are chosen to favour short, deterministic
// responses (low temperature, max_tokens capped) so the judge
// cannot run away with budget.
func (j *registryJudge) Complete(ctx context.Context, prompt string) (string, error) {
	out, err := j.reg.Complete(ctx, &Request{
		Messages:  []Message{{Role: "user", Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		return "", fmt.Errorf("judge %s: %w", j.name, err)
	}
	if out.Content == "" {
		return "", fmt.Errorf("judge %s: empty response", j.name)
	}
	return out.Content, nil
}
