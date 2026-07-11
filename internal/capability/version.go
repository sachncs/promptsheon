package capability

import "time"

// Version is where engineering happens.
//
// Each version is immutable after creation. A Version is identified
// by its Manifest — a value-typed, content-addressed composition
// of leaf artifacts (Prompt, ModelPolicy, RuntimePolicy,
// ContextContract, Memory, plus zero-or-more Guardrails, Tools,
// KnowledgeSources, MCPServers). The Manifest hash (sha256 over
// canonical encoding) is the Version's primary identity; the
// integer `Version` is a display counter that increments only
// when the Manifest changes.
//
// ADR-0010 / v0.1.0 forward-only: the legacy embedded-bundle
// fields are gone. The Manifest is the single source of truth;
// producers and consumers that previously read Prompt /
// ModelPolicy / etc. now read the corresponding ArtifactRef from
// the Manifest (or fetch the artifact's bytes via the
// content-addressable store by the recorded hash).
//
// This is analogous to a Docker image or a Kubernetes Deployment
// spec: a self-contained, versioned, immutable snapshot of the
// implementation.
type Version struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	Version      int       `json:"version"`
	Manifest     Manifest  `json:"manifest"`
	ManifestHash string    `json:"manifest_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
}
