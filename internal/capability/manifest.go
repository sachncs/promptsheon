package capability

import (
	"errors"
	"fmt"
)

// ArtifactKind classifies a content-addressed reference inside a Manifest.
//
// Each artifact is stored in the content-addressable store as immutable
// bytes addressed by its SHA-256 hash. A Version's Manifest is a pure
// composition of artifact references; the bytes of the artifacts never
// get embedded into the Version itself.
type ArtifactKind string

const (
	ArtifactPrompt        ArtifactKind = "prompt"
	ArtifactModelPolicy   ArtifactKind = "model_policy"
	ArtifactRuntimePolicy ArtifactKind = "runtime_policy"
	ArtifactContext       ArtifactKind = "context_contract"
	ArtifactMemory        ArtifactKind = "memory"
	ArtifactKnowledge     ArtifactKind = "knowledge_source"
	ArtifactGuardrail     ArtifactKind = "guardrail"
	ArtifactTool          ArtifactKind = "tool"
	ArtifactMCPServer     ArtifactKind = "mcp_server"
)

// ArtifactRef is a content-addressed pointer to an immutable artifact.
//
// The combination of (Kind, Hash) uniquely identifies the bytes of one
// artifact. Hash is the SHA-256 of the artifact's canonical encoding,
// matching the CAS scheme defined by ADR-0001.
type ArtifactRef struct {
	Kind ArtifactKind `json:"kind"`
	Hash string       `json:"hash"`
}

// Valid reports whether the reference is well-formed.
func (r ArtifactRef) Valid() error {
	switch r.Kind {
	case ArtifactPrompt, ArtifactModelPolicy, ArtifactRuntimePolicy,
		ArtifactContext, ArtifactMemory, ArtifactKnowledge,
		ArtifactGuardrail, ArtifactTool, ArtifactMCPServer:
	default:
		return fmt.Errorf("artifact: unknown kind %q", r.Kind)
	}
	if len(r.Hash) != 64 {
		return fmt.Errorf("artifact: hash must be 64 hex characters (sha-256), got %d", len(r.Hash))
	}
	for _, b := range r.Hash {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')) {
			return fmt.Errorf("artifact: hash must be lowercase hex, got %q", r.Hash)
		}
	}
	return nil
}

// Manifest is a content-addressed composition of artifacts that defines
// the implementation of a Capability Version.
//
// It exists to solve version-explosion: under the legacy embedded Version
// bundle, changing a guardrail or a tool created a new "Version",
// destroying the meaning of version numbers. A Manifest names each
// artifact by hash so guardrails, tools, MCP servers, and knowledge
// sources can evolve independently without renaming the Capability
// Version they belong to.
//
// The list slices (Guardrails, Tools, Knowledge, MCPServers) are
// deliberately ordered and deduplicated by hash: deterministic order
// keeps hash(M manifest) reproducible, and dedupe keeps replay buffers
// deduplicated by content.
//
// Manifest is a value type. Mutation produces a new value; the original
// is left untouched.
type Manifest struct {
	Prompt        ArtifactRef   `json:"prompt"`
	ModelPolicy   ArtifactRef   `json:"model_policy"`
	RuntimePolicy ArtifactRef   `json:"runtime_policy"`
	Context       ArtifactRef   `json:"context_contract"`
	Memory        ArtifactRef   `json:"memory"`
	Guardrails    []ArtifactRef `json:"guardrails,omitempty"`
	Tools         []ArtifactRef `json:"tools,omitempty"`
	Knowledge     []ArtifactRef `json:"knowledge_sources,omitempty"`
	MCPServers    []ArtifactRef `json:"mcp_servers,omitempty"`
}

// ErrEmptyManifest indicates a Manifest that has not been populated.
// An empty Manifest must never be deployed, approved, or evaluated.
var ErrEmptyManifest = errors.New("manifest is empty")

// Validate checks structural correctness of the Manifest.
//
// It does not check whether the referenced artifacts exist; that is a
// CAS lookup, not a domain invariant. Domain invariants enforced here
// are the ones that make a Manifest meaningful: a Manifest must
// identify its three core artifacts (prompt, model policy, runtime
// policy) and may not contain duplicate hashes within a single slice.
func (m Manifest) Validate() error {
	if err := m.Prompt.Valid(); err != nil {
		return fmt.Errorf("manifest: prompt: %w", err)
	}
	if err := m.ModelPolicy.Valid(); err != nil {
		return fmt.Errorf("manifest: model_policy: %w", err)
	}
	if err := m.RuntimePolicy.Valid(); err != nil {
		return fmt.Errorf("manifest: runtime_policy: %w", err)
	}
	if err := m.Context.Valid(); err != nil {
		return fmt.Errorf("manifest: context_contract: %w", err)
	}
	if err := m.Memory.Valid(); err != nil {
		return fmt.Errorf("manifest: memory: %w", err)
	}
	if err := validateSlice(m.Guardrails, "guardrails"); err != nil {
		return err
	}
	if err := validateSlice(m.Tools, "tools"); err != nil {
		return err
	}
	if err := validateSlice(m.Knowledge, "knowledge_sources"); err != nil {
		return err
	}
	if err := validateSlice(m.MCPServers, "mcp_servers"); err != nil {
		return err
	}
	if m.Prompt.Hash == "" &&
		m.ModelPolicy.Hash == "" &&
		m.RuntimePolicy.Hash == "" &&
		m.Context.Hash == "" &&
		m.Memory.Hash == "" {
		return ErrEmptyManifest
	}
	return nil
}

func validateSlice(refs []ArtifactRef, field string) error {
	seen := make(map[string]struct{}, len(refs))
	for i, r := range refs {
		if err := r.Valid(); err != nil {
			return fmt.Errorf("manifest: %s[%d]: %w", field, i, err)
		}
		if _, dup := seen[r.Hash]; dup {
			return fmt.Errorf("manifest: %s contains duplicate hash %s", field, r.Hash)
		}
		seen[r.Hash] = struct{}{}
	}
	return nil
}
