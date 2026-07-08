package capability

// ContextContract defines the boundaries and rules for context assembly.
//
// The runtime validates this contract before execution — ensuring every
// required context item is present, no forbidden items leak through, and
// the total size stays within budget.
type ContextContract struct {
	RequiredContext     []ContextRef `json:"required_context,omitempty"`
	OptionalContext     []ContextRef `json:"optional_context,omitempty"`
	ForbiddenContext    []string     `json:"forbidden_context,omitempty"` // keys / patterns
	MaximumSize         int          `json:"maximum_size"`                // in tokens
	CompressionStrategy string       `json:"compression_strategy,omitempty"`
	RetrievalStrategy   string       `json:"retrieval_strategy,omitempty"` // semantic, keyword, hybrid
}

// ContextRef references a piece of context by key, with an optional source.
type ContextRef struct {
	Key    string `json:"key"`
	Source string `json:"source,omitempty"` // e.g. "session", "document", "memory", "tool_output"
}
