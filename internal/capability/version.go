package capability

import "time"

// Version is where engineering happens.
//
// Each version is immutable after creation. A Version is identified by
// its Manifest — a value-typed, content-addressed composition of leaf
// artifacts (Prompt, ModelPolicy, RuntimePolicy, ContextContract,
// Memory, plus zero-or-more Guardrails, Tools, KnowledgeSources,
// MCPServers). The Manifest hash (sha256 over canonical encoding) is
// the Version's primary identity; the integer `Version` is a display
// counter that increments only when the Manifest changes.
//
// ADR-0010. The legacy embedded-fields representation is retained on
// this struct during the transition window so older callers continue
// to compile; new code should construct Versions with Manifest set and
// the legacy fields left zero.
//
// This is analogous to a Docker image or a Kubernetes Deployment
// spec: a self-contained, versioned, immutable snapshot of the
// implementation.
type Version struct {
	ID              string            `json:"id"`
	CapabilityID    string            `json:"capability_id"`
	Version         int               `json:"version"`
	Manifest        Manifest          `json:"manifest"`
	ManifestHash    string            `json:"manifest_hash,omitempty"`
	// Legacy fields retained during the migration; new code should
	// not read or write them. Set by the storage layer from the
	// embedded JSON columns when the row was written before
	// migration 023.
	Prompt          Prompt            `json:"prompt,omitempty"`
	ModelPolicy     ModelPolicy       `json:"model_policy,omitempty"`
	ContextContract ContextContract   `json:"context_contract,omitempty"`
	Knowledge       []KnowledgeSource `json:"knowledge,omitempty"`
	Memory          MemoryConfig      `json:"memory,omitempty"`
	Guardrails      []Guardrail       `json:"guardrails,omitempty"`
	Tools           []Tool            `json:"tools,omitempty"`
	MCPServers      []MCPServer       `json:"mcp_servers,omitempty"`
	RuntimePolicy   RuntimePolicy     `json:"runtime_policy,omitempty"`
	EvaluationSuite EvaluationSuite   `json:"evaluation_suite,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	CreatedBy       string            `json:"created_by"`
}
