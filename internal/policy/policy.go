// Package policy defines Workspace-level Policies that constrain
// Capability Versions, Releases, and Executions.
//
// A Policy is a per-Workspace value (not a per-Version value). Version
// scope covers "how does this Capability behave"; Workspace scope
// covers "what is allowed at all in this Workspace". Examples are
// "PII must be redacted", "only these models may be referenced", and
// "no production executions may cost more than $1".
//
// Putting the type here rather than inside the Version aggregate is
// what enables these constraints to apply across Versions — without
// it, a guardrail fix is a policy escape hatch.
//
// Policies are evaluated against the inputs that the runtime cares
// about (an Execution request, a Release, a Manifest). They return
// an Effect that the runtime then honors.
package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Effect is what the runtime does in response to a Policy decision.
type Effect string

const (
	// EffectAllow lets the action proceed unchanged.
	EffectAllow Effect = "allow"
	// EffectDeny blocks the action and surfaces the Reason to the
	// caller. Deny is final for the action; the caller must request
	// again or escalate.
	EffectDeny Effect = "deny"
	// EffectRedact lets the action proceed but the Reason carries
	// a redaction directive the runtime must apply.
	EffectRedact Effect = "redact"
)

// Decision is the output of evaluating one Policy against one input.
type Decision struct {
	Effect Effect
	Reason string
	Policy string
}

// Allow constructs an Allow Decision.
func Allow(policy, reason string) Decision {
	return Decision{Effect: EffectAllow, Policy: policy, Reason: reason}
}

// Deny constructs a Deny Decision.
func Deny(policy, reason string) Decision {
	return Decision{Effect: EffectDeny, Policy: policy, Reason: reason}
}

// Redact constructs a Redact Decision.
func Redact(policy, reason string) Decision {
	return Decision{Effect: EffectRedact, Policy: policy, Reason: reason}
}

// Scope identifies what a Policy applies to. A Policy decides for one
// Scope; combining Policies to cover all Scopes is the caller's job.
type Scope string

const (
	ScopeManifest  Scope = "manifest"
	ScopeRelease   Scope = "release"
	ScopeExecution Scope = "execution"
	ScopeWorkspace Scope = "workspace"
)

// Policy is a single rule evaluated against a single input.
//
// Policy implementations must be deterministic: the same inputs must
// produce the same Decision on every call. This matters because
// Policy evaluation is recorded in audit logs and replay buffers;
// non-determinism would corrupt both.
type Policy interface {
	Name() string
	Scope() Scope
	Evaluate(input any) Decision
}

// ManifestPolicyInput carries enough information about a Manifest for
// a Policy to decide without reaching into the CAS.
type ManifestPolicyInput struct {
	WorkspaceID   string
	CapabilityID  string
	Model         string
	Provider      string
	EstimatedCost float64
	HasPII        bool
	DataRegion    string
}

// ReleasePolicyInput is the input for release-scoped policies.
type ReleasePolicyInput struct {
	WorkspaceID  string
	Environment  string
	SourceRef    string
	BypassWindow bool
	Time         time.Time
}

// ExecutionPolicyInput is the input for execution-scoped policies.
type ExecutionPolicyInput struct {
	WorkspaceID  string
	Environment  string
	CapabilityID string
	EstimatedUSD float64
	Time         time.Time
}

// AllProviders is the closed set of model providers the Workspace
// allows. An empty set means "no providers allowed" — there is no
// implicit "allow everything".
type AllowedProviders struct {
	Providers  map[string]struct{}
	PolicyName string
}

// Name implements Policy.
func (a AllowedProviders) Name() string { return a.PolicyName }

// Scope implements Policy.
func (a AllowedProviders) Scope() Scope { return ScopeManifest }

// Evaluate implements Policy.
func (a AllowedProviders) Evaluate(input any) Decision {
	mpi, ok := input.(ManifestPolicyInput)
	if !ok {
		return Allow(a.PolicyName, "input not manifest scope; deferred to caller")
	}
	if _, ok := a.Providers[mpi.Provider]; !ok {
		return Deny(a.PolicyName, fmt.Sprintf("provider %q is not in the workspace allowlist", mpi.Provider))
	}
	return Allow(a.PolicyName, "")
}

// CostCeiling denies any execution whose estimated cost exceeds the
// configured ceiling. Returning Deny on undefined input is correct:
// Policy evaluation must default closed, per the Project Philosophy.
type CostCeiling struct {
	MaxUSD     float64
	PolicyName string
}

// Name implements Policy.
func (c CostCeiling) Name() string { return c.PolicyName }

// Scope implements Policy.
func (c CostCeiling) Scope() Scope { return ScopeExecution }

// Evaluate implements Policy.
func (c CostCeiling) Evaluate(input any) Decision {
	epi, ok := input.(ExecutionPolicyInput)
	if !ok {
		return Deny(c.PolicyName, "cost ceiling requires execution input")
	}
	if epi.EstimatedUSD > c.MaxUSD {
		return Deny(c.PolicyName, fmt.Sprintf("estimated cost %.4f exceeds ceiling %.4f", epi.EstimatedUSD, c.MaxUSD))
	}
	return Allow(c.PolicyName, "")
}

// ChangeWindow denies Release promotions outside the configured
// promotion window. This is the implementation behind "no deploys on
// Friday night" governance.
type ChangeWindow struct {
	AllowFromHour int
	AllowToHour   int
	DaysOfWeek    map[time.Weekday]struct{}
	PolicyName    string
}

// NewChangeWindow constructs a ChangeWindow where every supplied
// Weekday is allowed and the hour-of-day range is half-open
// [FromHour, ToHour). Hour 0 means midnight UTC; hours wrap at 24.
func NewChangeWindow(policyName string, fromHour, toHour int, days ...time.Weekday) (ChangeWindow, error) {
	if fromHour < 0 || fromHour > 23 || toHour < 1 || toHour > 24 {
		return ChangeWindow{}, errors.New("policy: invalid hour range")
	}
	if fromHour >= toHour {
		return ChangeWindow{}, errors.New("policy: fromHour must be < toHour")
	}
	if len(days) == 0 {
		return ChangeWindow{}, errors.New("policy: at least one day of week is required")
	}
	m := make(map[time.Weekday]struct{}, len(days))
	for _, d := range days {
		m[d] = struct{}{}
	}
	return ChangeWindow{
		PolicyName:    policyName,
		AllowFromHour: fromHour,
		AllowToHour:   toHour,
		DaysOfWeek:    m,
	}, nil
}

// Name implements Policy.
func (c ChangeWindow) Name() string { return c.PolicyName }

// Scope implements Policy.
func (c ChangeWindow) Scope() Scope { return ScopeRelease }

// Evaluate implements Policy.
func (c ChangeWindow) Evaluate(input any) Decision {
	rpi, ok := input.(ReleasePolicyInput)
	if !ok {
		return Deny(c.PolicyName, "change window requires release input")
	}
	if rpi.BypassWindow {
		return Allow(c.PolicyName, "bypass flag set")
	}
	if rpi.Time.IsZero() {
		return Deny(c.PolicyName, "change window requires explicit time")
	}
	if _, ok := c.DaysOfWeek[rpi.Time.Weekday()]; !ok {
		return Deny(c.PolicyName, fmt.Sprintf("day %s is not in change window", rpi.Time.Weekday()))
	}
	hour := rpi.Time.Hour()
	if hour < c.AllowFromHour || hour >= c.AllowToHour {
		return Deny(c.PolicyName, fmt.Sprintf("hour %02d outside window %02d-%02d", hour, c.AllowFromHour, c.AllowToHour))
	}
	return Allow(c.PolicyName, "")
}

// PIIRedaction forces a Redact Decision whenever the input is marked
// as containing PII. The actual redaction is performed by a separate
// component; this Policy only signals intent.
type PIIRedaction struct {
	PolicyName string
}

// Name implements Policy.
func (p PIIRedaction) Name() string { return p.PolicyName }

// Scope implements Policy.
func (p PIIRedaction) Scope() Scope { return ScopeManifest }

// Evaluate implements Policy.
func (p PIIRedaction) Evaluate(input any) Decision {
	mpi, ok := input.(ManifestPolicyInput)
	if !ok {
		return Allow(p.PolicyName, "non-manifest input deferred")
	}
	if mpi.HasPII {
		return Redact(p.PolicyName, "manifest scoped as containing PII")
	}
	return Allow(p.PolicyName, "")
}

// DataResidency restricts Manifest evaluation to a configured
// workspace data region. The Region field is evaluated with the
// Input's DataRegion.
type DataResidency struct {
	Region     string
	PolicyName string
}

// Name implements Policy.
func (d DataResidency) Name() string { return d.PolicyName }

// Scope implements Policy.
func (d DataResidency) Scope() Scope { return ScopeManifest }

// Evaluate implements Policy.
func (d DataResidency) Evaluate(input any) Decision {
	mpi, ok := input.(ManifestPolicyInput)
	if !ok {
		return Deny(d.PolicyName, "data residency requires manifest input")
	}
	if mpi.DataRegion != d.Region {
		return Deny(d.PolicyName, fmt.Sprintf("data region %q not in workspace region %q", mpi.DataRegion, d.Region))
	}
	return Allow(d.PolicyName, "")
}

// Bundle collects Policies by Scope. Evaluation short-circuits on
// the first Deny, so order matters: deny-by-default policies come
// first to fail closed.
type Bundle struct {
	Manifest  []Policy
	Release   []Policy
	Execution []Policy
}

// EvaluateManifest runs every Manifest Policy and returns the first
// Deny, the first Redact, or Allow if all succeed.
func (b Bundle) EvaluateManifest(in ManifestPolicyInput) Decision {
	for _, p := range b.Manifest {
		if d := p.Evaluate(in); d.Effect != EffectAllow {
			return d
		}
	}
	return Allow("policy.Bundle", "")
}

// EvaluateRelease runs every Release Policy.
func (b Bundle) EvaluateRelease(in ReleasePolicyInput) Decision {
	for _, p := range b.Release {
		if d := p.Evaluate(in); d.Effect != EffectAllow {
			return d
		}
	}
	return Allow("policy.Bundle", "")
}

// EvaluateExecution runs every Execution Policy.
func (b Bundle) EvaluateExecution(in ExecutionPolicyInput) Decision {
	for _, p := range b.Execution {
		if d := p.Evaluate(in); d.Effect != EffectAllow {
			return d
		}
	}
	return Allow("policy.Bundle", "")
}

// Compile-time check: capability.Manifest is the value the caller has
// but Policy inputs are domain-shaped. This indirection prevents an
// accidental coupling between Workspace Policy and Version aggregate.
var _ = capability.ArtifactPrompt
