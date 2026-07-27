// Package reasoning implements the Reasoning Compiler — the
// primitive that turns an Intent (a goal + constraints) into an
// executable CapabilityPlan (a DAG of Capability invocations).
//
// The compiler is the unit of composition: it walks the
// capability catalog, picks capabilities that satisfy the
// intent's preconditions, and produces a topologically-sorted
// plan the workflow Engine can run end-to-end.
//
// The compiler is deliberately deterministic for a given
// (intent, catalog, registry) tuple. Non-determinism lives in
// the workflow Engine (LLM calls have variance) and in the
// bandit (which arm to pick); the compiler itself is pure.
package reasoning

import (
	"github.com/sachncs/promptsheon/backend"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

)

// Intent is the user's high-level goal. The Compiler translates
// an Intent into a CapabilityPlan. An Intent is the input to
// the compiler; a CapabilityPlan is the output.
type Intent struct {
	ID            string         `json:"id"`
	Goal          string         `json:"goal"`
	Inputs        map[string]any `json:"inputs"`
	Constraints   Constraints    `json:"constraints"`
	Tags          []string       `json:"tags,omitempty"`
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

// Constraints are the cost / latency / quality bounds the
// compiled plan must satisfy. The Compiler rejects capabilities
// that violate any constraint; the engine rejects at runtime.
type Constraints struct {
	MaxCostUSD     float64   `json:"max_cost_usd,omitempty"`
	MaxLatencyMS   int64     `json:"max_latency_ms,omitempty"`
	MinTrustScore  float64   `json:"min_trust_score,omitempty"`
	Deadline       time.Time `json:"deadline,omitempty"`
	BudgetTokenCap int       `json:"budget_token_cap,omitempty"`
	RequiredTags   []string  `json:"required_tags,omitempty"`
}

// CapabilityDescriptor is the catalog's view of a Capability.
// The Compiler consumes descriptors (not the live Capability
// aggregate) so the catalog surface stays thin.
type CapabilityDescriptor struct {
	ID         string
	Name       string
	Tags       []string
	TrustScore float64
	CostUSD    float64
	LatencyMS  int64
	Inputs     []string // required input names; satisfied by Intent.Inputs or prior step outputs.
	Outputs    []string // names of values produced for downstream steps.
}

// Plan is the compiled output: a DAG of capabilities with
// input/output bindings and a topological execution order.
// Steps execute in topological order; downstream steps
// receive upstream outputs via the bind map.
type Plan struct {
	ID         string        `json:"id"`
	IntentID   string        `json:"intent_id"`
	Steps      []Step        `json:"steps"`
	BudgetCost float64       `json:"budget_cost"`
	BudgetETA  time.Duration `json:"budget_eta_ms"`
	CreatedAt  time.Time     `json:"created_at"`
}

// Step is one capability invocation in the plan. Order is
// implicit in the parent's Steps slice (topological).
type Step struct {
	ID           string            `json:"id"`
	CapabilityID string            `json:"capability_id"`
	Inputs       map[string]string `json:"inputs"` // input name -> prior step ID or "intent" sentinel.
	DependsOn    []string          `json:"depends_on,omitempty"`
}

// backend.ErrorReasoningNoMatch is returned by Compile when no CapabilityDescriptor
// satisfies the Intent. Callers should map this to a 422.

// backend.ErrorReasoningConstraintViolation is returned by Compile when the only
// candidates exceed one of the Intent's Constraints.

// Compiler turns an Intent into a Plan. The Compiler is a
// pure function: same input, same output, no side effects.
type Compiler struct {
	// Catalog is the read-only list of available capabilities.
	// Production wiring passes the Workspace's catalog; tests
	// pass a curated slice.
	Catalog []CapabilityDescriptor
}

// NewCompiler constructs a Compiler with the supplied catalog.
// A nil catalog is permitted; Compile returns backend.ErrorReasoningNoMatch for
// every intent.
func NewCompiler(catalog []CapabilityDescriptor) *Compiler {
	return &Compiler{Catalog: catalog}
}

// Compile turns an Intent into a Plan. The algorithm is:
//
//  1. Match the catalog by Intent.Goal + Intent.Tags (semantic
//     intent match). If nothing matches, return backend.ErrorReasoningNoMatch.
//  2. Filter surviving candidates by Constraints (cost,
//     latency, trust, tags). If everything is filtered out by
//     constraints, return backend.ErrorReasoningConstraintViolation.
//  3. Pick the highest-scoring candidate. Bind inputs.
//  4. The plan has one step for now; the multi-step DAG is a
//     v0.4.0 follow-on.
//
// The returned Plan is value-immutable: subsequent mutations
// to the Compiler's catalog do not affect already-compiled plans.
func (c *Compiler) Compile(ctx context.Context, intent Intent) (*Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(intent.Goal) == "" {
		return nil, fmt.Errorf("reasoning: empty goal")
	}
	matched := c.matchIntent(intent)
	if len(matched) == 0 {
		return nil, backend.ErrorReasoningNoMatch
	}
	filtered := c.applyConstraints(intent, matched)
	if len(filtered) == 0 {
		return nil, backend.ErrorReasoningConstraintViolation
	}
	scored := c.score(intent, filtered)
	if len(scored) == 0 {
		return nil, backend.ErrorReasoningConstraintViolation
	}
	// Pick the highest-scoring candidate.
	best := scored[0]
	plan := &Plan{
		ID:        "plan-" + intent.ID,
		IntentID:  intent.ID,
		CreatedAt: time.Now().UTC(),
		Steps: []Step{
			{
				ID:           "step-1",
				CapabilityID: best.ID,
				Inputs:       bindInputs(intent.Inputs, best.Inputs),
				DependsOn:    nil,
			},
		},
		BudgetCost: best.CostUSD,
		BudgetETA:  time.Duration(best.LatencyMS) * time.Millisecond,
	}
	if plan.BudgetETA == 0 {
		plan.BudgetETA = time.Second
	}
	return plan, nil
}

// matchIntent does the goal + tag semantic match. A capability
// survives if EITHER its name shares a token with the goal OR
// its tag set intersects (non-empty) with the intent's Tags.
// Empty intent.Tags + empty goal matches nothing; that
// returns backend.ErrorReasoningNoMatch, which is the desired behaviour for a
// missing or vague Intent.
func (c *Compiler) matchIntent(intent Intent) []CapabilityDescriptor {
	intentTags := make(map[string]struct{}, len(intent.Tags))
	for _, t := range intent.Tags {
		intentTags[t] = struct{}{}
	}
	var out []CapabilityDescriptor
	for _, cap := range c.Catalog {
		matchedByGoal := intentMatches(cap, intent)
		matchedByTags := len(intentTags) > 0 && tagSetIntersects(cap.Tags, intentTags)
		if matchedByGoal || matchedByTags {
			out = append(out, cap)
		}
	}
	return out
}

func tagSetIntersects(a []string, b map[string]struct{}) bool {
	for _, t := range a {
		if _, ok := b[t]; ok {
			return true
		}
	}
	return false
}

// applyConstraints drops capabilities that violate the
// Intent's Constraints. A capability that violates any single
// constraint is dropped.
func (c *Compiler) applyConstraints(intent Intent, candidates []CapabilityDescriptor) []CapabilityDescriptor {
	required := map[string]struct{}{}
	for _, t := range intent.Constraints.RequiredTags {
		required[t] = struct{}{}
	}
	var out []CapabilityDescriptor
	for _, cap := range candidates {
		if intent.Constraints.MinTrustScore > 0 && cap.TrustScore < intent.Constraints.MinTrustScore {
			continue
		}
		if intent.Constraints.MaxCostUSD > 0 && cap.CostUSD > intent.Constraints.MaxCostUSD {
			continue
		}
		if intent.Constraints.MaxLatencyMS > 0 && cap.LatencyMS > intent.Constraints.MaxLatencyMS {
			continue
		}
		if !hasAllTags(cap.Tags, required) {
			continue
		}
		out = append(out, cap)
	}
	return out
}

// score ranks candidates by fit. The score combines tag
// overlap (high weight), goal-token overlap (medium), and a
// small penalty for cost so an expensive candidate loses to a
// cheaper one with equal fit.
func (c *Compiler) score(intent Intent, candidates []CapabilityDescriptor) []CapabilityDescriptor {
	type scored struct {
		cap  CapabilityDescriptor
		note float64
	}
	ranks := make([]scored, 0, len(candidates))
	intentTokens := tokenise(intent.Goal)
	for _, cap := range candidates {
		tagScore := tagOverlap(cap.Tags, intent.Tags)
		tokenScore := tokenOverlap(tokenise(cap.Name), intentTokens)
		// Cheap penalty so equal-fit candidates resolve to
		// the cheaper one. Cost is in USD; penalty is small
		// so it does not override a tag-overlap win.
		penalty := cap.CostUSD * 0.01
		ranks = append(ranks, scored{cap: cap, note: tagScore*2 + tokenScore - penalty})
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		return ranks[i].note > ranks[j].note
	})
	out := make([]CapabilityDescriptor, len(ranks))
	for i, r := range ranks {
		out[i] = r.cap
	}
	return out
}

// bindInputs produces the input-binding map for a step. Each
// required input name is mapped to "intent" if the intent
// supplies it, otherwise to the upstream step ID that produces
// the input. Single-step plans bind everything to "intent".
func bindInputs(provided map[string]any, required []string) map[string]string {
	out := make(map[string]string, len(required))
	for _, name := range required {
		if _, ok := provided[name]; ok {
			out[name] = "intent"
			continue
		}
		out[name] = ""
	}
	return out
}

func hasAllTags(have []string, required map[string]struct{}) bool {
	if len(required) == 0 {
		return true
	}
	haveSet := make(map[string]struct{}, len(have))
	for _, t := range have {
		haveSet[t] = struct{}{}
	}
	for t := range required {
		if _, ok := haveSet[t]; !ok {
			return false
		}
	}
	return true
}

// intentMatches is the coarse filter: does the capability's
// name share any token with the goal? The full semantic match
// (LLM-reranked) is a future enhancement; for v0.3.0 the
// substring heuristic is the deterministic baseline.
func intentMatches(cap CapabilityDescriptor, intent Intent) bool {
	capTokens := tokenise(cap.Name)
	intentTokens := tokenise(intent.Goal)
	return tokenOverlap(capTokens, intentTokens) > 0
}

func tokenise(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		// Strip simple punctuation; the goal string is
		// free-form English and we don't need a real lexer.
		tok = strings.Trim(tok, ".,;:()[]{}\"'")
		if tok == "" {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

func tagOverlap(have, want []string) float64 {
	if len(want) == 0 {
		return 0
	}
	haveSet := make(map[string]struct{}, len(have))
	for _, t := range have {
		haveSet[t] = struct{}{}
	}
	hits := 0
	for _, t := range want {
		if _, ok := haveSet[t]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(want))
}

func tokenOverlap(a, b map[string]struct{}) float64 {
	if len(b) == 0 {
		return 0
	}
	hits := 0
	for t := range b {
		if _, ok := a[t]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(b))
}
