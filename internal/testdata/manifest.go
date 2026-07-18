// Package testdata provides shared, zero-dependency test fixtures
// used by _test.go files in any package. The package intentionally
// imports only the capability domain types so consumers can link
// it without dragging in the storage layer or other dependencies.
package testdata

import "github.com/sachncs/promptsheon/internal/capability"

// SampleManifestHash is a 32-byte (64 hex) SHA-256 placeholder used
// by test fixtures. Callers should use NewManifest to build a manifest
// whose ArtifactRefs point at this hash; release tests rely on
// manifest.Validate() passing, which requires non-empty 64-hex hashes.
const SampleManifestHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// NewManifest returns a capability.Manifest whose five required
// ArtifactRefs (Prompt, ModelPolicy, RuntimePolicy, Context, Memory)
// all point at SampleManifestHash. Use in place of ad-hoc helpers
// scattered across *_test.go files.
func NewManifest() capability.Manifest {
	h := SampleManifestHash
	return capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: h},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: h},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: h},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: h},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: h},
	}
}
