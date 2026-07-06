package capability

import "time"

// CapabilityVersion is where engineering happens.
//
// Each version is immutable after creation. It bundles every artifact needed
// to define, execute, and evaluate a capability — the prompt, model policy,
// context contract, guardrails, tools, MCP servers, knowledge sources,
// memory config, runtime policy, and evaluation suite.
//
// This is analogous to a Docker image or a Kubernetes Deployment spec:
// a self-contained, versioned, immutable snapshot of the implementation.
type CapabilityVersion struct {
	ID              string           `json:"id"`
	CapabilityID    string           `json:"capability_id"`
	Version         int              `json:"version"`
	Prompt          Prompt           `json:"prompt"`
	ModelPolicy     ModelPolicy      `json:"model_policy"`
	ContextContract ContextContract  `json:"context_contract"`
	Knowledge       []KnowledgeSource `json:"knowledge,omitempty"`
	Memory          MemoryConfig     `json:"memory"`
	Guardrails      []Guardrail      `json:"guardrails,omitempty"`
	Tools           []Tool           `json:"tools,omitempty"`
	MCPServers      []MCPServer      `json:"mcp_servers,omitempty"`
	RuntimePolicy   RuntimePolicy    `json:"runtime_policy"`
	EvaluationSuite EvaluationSuite  `json:"evaluation_suite"`
	CreatedAt       time.Time        `json:"created_at"`
	CreatedBy       string           `json:"created_by"`
}
