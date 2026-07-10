# ADR 0010: Capability Version is a Manifest, not a Bundle

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

`internal/capability/version.go` defines `Version` as a struct that embeds every artifact needed to execute a Capability:

```go
type Version struct {
    Prompt            Prompt
    ModelPolicy       ModelPolicy
    ContextContract   ContextContract
    Knowledge         []KnowledgeSource
    Memory            MemoryConfig
    Guardrails        []Guardrail
    Tools             []Tool
    MCPServers        []MCPServer
    RuntimePolicy     RuntimePolicy
    EvaluationSuite   EvaluationSuite
    // ...
}
```

This is a *bundle*: changing any leaf artifact creates a new `Version`. In a system that resolves a Capability by its `current_version_id`, that has two consequences:

1. **Version-explosion.** Fixing a guardrail typo causes Capability "Invoice Extraction" to leave v17 and arrive at v17.43.118. Customers lose the meaning of version numbers, lose dashboards, lose rollback granularity.
2. **No immutable reuse.** A guardrail that is shared by ten Capabilities cannot be referenced; it is copied into each `Version`. A guardrail hot-fix has to happen ten times.

The system already has a content-addressable store (`internal/promptsheon/`, ADR-0001) that can address each artifact by SHA-256. The CAS is the right substrate for a Manifest.

## Decision

A Capability Version is a **Manifest**: a value-typed composition of content-addressed references to its leaf artifacts.

```go
type ArtifactRef struct {
    Kind ArtifactKind // prompt, model_policy, runtime_policy, ...
    Hash string       // 64-char lowercase hex, sha-256
}

type Manifest struct {
    Prompt        ArtifactRef
    ModelPolicy   ArtifactRef
    RuntimePolicy ArtifactRef
    Context       ArtifactRef
    Memory        ArtifactRef
    Guardrails    []ArtifactRef
    Tools         []ArtifactRef
    Knowledge     []ArtifactRef
    MCPServers    []ArtifactRef
}
```

Manifest is **value-immutable**. Methods that appear to mutate return new values. Reproducibility becomes a Manifest hash.

The legacy embedded struct in `version.go` remains for one release. New code paths construct `Version` from a `Manifest`. After one release the legacy fields are removed.

## Options considered

1. **Manifest of content-addressed refs (chosen).** Solves version-explosion; guards-and-tools get their own independent history; one Manifest hash = reproducible execution.
2. **Sub-versions per artifact.** Version 17.43.118 looks identical to a bundle but is harder to query. **Rejected** — it confirms version-explosion rather than curing it.
3. **Keep the bundle.** Status quo. **Rejected** — version-explosion is already a problem in the system and gets worse as the artifact list grows.

## Consequences

Positive:

- A Capability's `Version` only increments when the *composed behavior* changes.
- Guardrails, tools, MCP servers, and knowledge sources have their own CAS history and can be patched in place.
- Replay buffers can key on `ManifestHash` without needing to re-resolve the bundle.
- Lineage between Versions records "what changed" rather than "what was rewritten".

Negative:

- Every artifact that was previously inline must now go through the CAS. Old artifacts in legacy storage need a one-time migration that hashes their content.
- The Manifest hash is a new derived field; callers that previously compared Versions by ID must learn to compare by Manifest hash for reproducibility.
- The store layout changes: a Version row now stores a Manifest JSON, not a set of columns. Migrations 022 and onwards will switch the schema.

## References

- `internal/capability/manifest.go` — Manifest value type
- `internal/capability/manifest_test.go` — validation tests
- `internal/release/release.go` — Release references Manifest
- `docs/adr/0001-use-cas-for-prompt-history.md` — CAS substrate
