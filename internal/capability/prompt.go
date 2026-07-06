package capability

// Prompt defines what the model is instructed to do.
//
// Prompt becomes surprisingly small in this model — it is purely about
// instruction, not about telemetry, deployments, latency, or history.
// Those concerns live in other artifacts (RuntimePolicy, Deployment, Execution).
type Prompt struct {
	Role          string           `json:"role,omitempty"`
	Instructions  string           `json:"instructions"`
	Examples      []PromptExample  `json:"examples,omitempty"`
	Variables     []PromptVariable `json:"variables,omitempty"`
	Template      string           `json:"template,omitempty"`
	LocaleVariants map[string]string `json:"locale_variants,omitempty"`
}

// PromptExample is a few-shot example included in the prompt.
type PromptExample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// PromptVariable defines a template variable that can be substituted at runtime.
type PromptVariable struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "bool"
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}
