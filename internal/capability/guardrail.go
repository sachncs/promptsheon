package capability

// GuardrailPhase identifies when a guardrail is evaluated.
type GuardrailPhase string

const (
	// GuardrailPhasePre is a pre-execution guardrail phase.
	GuardrailPhasePre GuardrailPhase = "pre"
	// GuardrailPhaseRuntime is a runtime guardrail phase.
	GuardrailPhaseRuntime GuardrailPhase = "runtime"
	// GuardrailPhasePost is a post-execution guardrail phase.
	GuardrailPhasePost GuardrailPhase = "post"
)

// Guardrail is an independent safety or quality policy artifact.
//
// Each guardrail is independently versioned, configured, and measured.
// Guardrails execute at specific phases of the execution lifecycle:
// pre-execution (input validation), runtime (in-flight checks), and
// post-execution (output validation).
type Guardrail struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Phase     GuardrailPhase `json:"phase"`
	Version   string         `json:"version"`
	Config    map[string]any `json:"config,omitempty"`
	Threshold float64        `json:"threshold,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
	Severity  string         `json:"severity,omitempty"` // "low", "medium", "high", "critical"
}
