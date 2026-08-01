package capability

import (
	"fmt"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// BlastRadius is the impact classification of a Capability.
// Low blast-radius Capabilities can be auto-promoted by the
// Recommendation engine without human approval; medium and
// high always require a Maker/Checker quorum.
type BlastRadius string

const (
	BlastLow    BlastRadius = "low"
	BlastMedium BlastRadius = "medium"
	BlastHigh   BlastRadius = "high"
)

// Valid reports whether the BlastRadius is one of the closed
// set. The closed set is intentional: any new value is a
// control-plane decision, not a configuration change.
func (b BlastRadius) Valid() bool {
	switch b {
	case BlastLow, BlastMedium, BlastHigh:
		return true
	default:
		return false
	}
}

// errs.ErrInvalidBlastRadius is returned by Contract.Validate when
// the supplied BlastRadius is not in the closed set.

// errs.ErrEmptyContract is returned when a Capability has no
// Contract. The contract-less state is preserved for back-
// compat with v0.2.0 Capabilities that predate the Contract
// primitive; new Capabilities SHOULD attach a Contract.

// CapabilityContract is the typed contract attached to a
// Capability. A Capability with a contract declares what it
// promises to operators: an input schema, an output schema,
// success criteria, an SLO target, and a blast radius.
//
// The contract is the unit of governance. A Capability
// without a contract cannot be auto-promoted by the
// Recommendation engine — there is no blast radius to compare
// against the auto-adopt threshold.
type CapabilityContract struct {
	InputSchema    map[string]any `json:"input_schema,omitempty"`
	OutputSchema   map[string]any `json:"output_schema,omitempty"`
	SuccessRubric  string         `json:"success_rubric,omitempty"`
	SLOTarget      SLOTarget      `json:"slo_target,omitempty"`
	BlastRadius    BlastRadius    `json:"blast_radius"`
	AutoPromotable bool           `json:"auto_promotable"`
}

// SLOTarget is the success criteria the Capability promises to
// meet. Empty SLOTarget means the operator has not declared a
// target; the Recommendation engine treats empty as "no
// signal" and skips auto-promotion.
type SLOTarget struct {
	// MaxP95LatencyMS is the upper bound on the rolling p95
	// invoke latency. Zero means "no constraint".
	MaxP95LatencyMS int `json:"max_p95_latency_ms,omitempty"`
	// MinSuccessRate is the lower bound on the rolling success
	// rate (0.0..1.0). Zero means "no constraint".
	MinSuccessRate float64 `json:"min_success_rate,omitempty"`
	// MaxHallucinationRate is the upper bound on the rolling
	// hallucination rate. Zero means "no constraint".
	MaxHallucinationRate float64 `json:"max_hallucination_rate,omitempty"`
}

// Validate checks structural correctness of a
// CapabilityContract. Empty contracts are an error: a
// Capability that ships a Contract must declare at least a
// blast radius.
func (c CapabilityContract) Validate() error {
	if c.IsZero() {
		return errs.ErrEmptyContract
	}
	if !c.BlastRadius.Valid() {
		return fmt.Errorf("%w: %q", errs.ErrInvalidBlastRadius, c.BlastRadius)
	}
	if c.SLOTarget.MinSuccessRate < 0 || c.SLOTarget.MinSuccessRate > 1 {
		return fmt.Errorf("capability: success rate out of range: %f", c.SLOTarget.MinSuccessRate)
	}
	if c.SLOTarget.MaxHallucinationRate < 0 || c.SLOTarget.MaxHallucinationRate > 1 {
		return fmt.Errorf("capability: hallucination rate out of range: %f", c.SLOTarget.MaxHallucinationRate)
	}
	return nil
}

// IsZero reports whether the contract is the zero value.
// CapabilityContract contains maps and so cannot be compared
// with ==; this method walks the fields explicitly.
func (c CapabilityContract) IsZero() bool {
	return c.BlastRadius == "" &&
		c.SuccessRubric == "" &&
		len(c.InputSchema) == 0 &&
		len(c.OutputSchema) == 0 &&
		c.SLOTarget == SLOTarget{}
}

// CanAutoAdopt reports whether the contract permits the
// Recommendation engine to auto-promote. Low blast radius is
// auto-promotable only when the contract declares an SLO; medium
// and high require both an SLO and the operator's AutoPromotable
// opt-in.
//
// An empty SLO produces no auto-promotion signal: a capability
// without a quality or latency target cannot tell whether its
// output is getting better or worse, so blindly auto-promoting
// it would silently regress production.
func (c CapabilityContract) CanAutoAdopt() bool {
	if err := c.Validate(); err != nil {
		return false
	}
	if c.SLOTarget == (SLOTarget{}) {
		return false
	}
	switch c.BlastRadius {
	case BlastLow:
		return true
	case BlastMedium, BlastHigh:
		return c.AutoPromotable
	default:
		return false
	}
}
