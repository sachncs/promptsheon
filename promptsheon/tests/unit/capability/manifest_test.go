//go:build tests_migration


package capability_test

import (
	"testing"

	. "github.com/sachncs/promptsheon/promptsheon/capability"
)

func TestArtifactRefValid(t *testing.T) {
	t.Parallel()
	good := ArtifactRef{Kind: ArtifactPrompt, Hash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}
	if err := good.Valid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestArtifactRefValidationErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ref  ArtifactRef
	}{
		{"unknown kind", ArtifactRef{Kind: "widget", Hash: goodHash()}},
		{"short hash", ArtifactRef{Kind: ArtifactPrompt, Hash: "abc"}},
		{"uppercase hex", ArtifactRef{Kind: ArtifactPrompt, Hash: "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"}},
		{"non-hex char", ArtifactRef{Kind: ArtifactPrompt, Hash: "zbcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.ref.Valid(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestManifestValid(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Prompt:        ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
		ModelPolicy:   ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()},
		RuntimePolicy: ArtifactRef{Kind: ArtifactRuntimePolicy, Hash: goodHash()},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid manifest, got %v", err)
	}
}

// TestManifestMinimalThree pins MAN-1: a Manifest with only the
// three required artifacts (Prompt, ModelPolicy, RuntimePolicy)
// is valid. ContextContract and Memory are optional; the
// Resolver does not load them in v0.2.0. The previous design
// required all five, forcing every test fixture to fabricate
// empty hashes for fields the system never used.
func TestManifestMinimalThree(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Prompt:        ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
		ModelPolicy:   ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()},
		RuntimePolicy: ArtifactRef{Kind: ArtifactRuntimePolicy, Hash: goodHash()},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("minimal 3-artifact manifest must validate, got %v", err)
	}
}

// TestManifestWithOptionalsStillValid pins that adding
// ContextContract or Memory to a Manifest does not break
// validation; the optional fields remain valid ArtifactKinds.
func TestManifestWithOptionalsStillValid(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Prompt:        ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
		ModelPolicy:   ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()},
		RuntimePolicy: ArtifactRef{Kind: ArtifactRuntimePolicy, Hash: goodHash()},
		Context:       ArtifactRef{Kind: ArtifactContext, Hash: goodHash()},
		Memory:        ArtifactRef{Kind: ArtifactMemory, Hash: goodHash()},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest with optional context+memory must validate, got %v", err)
	}
}

func TestManifestEmpty(t *testing.T) {
	t.Parallel()
	if err := (Manifest{}).Validate(); err == nil {
		t.Fatalf("expected errs.ErrEmptyManifest")
	}
}

func TestManifestMissingCoreArtifact(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Prompt: ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
	}
	if err := m.Validate(); err == nil {
		t.Fatalf("expected error for missing model_policy")
	}
}

func TestManifestDuplicateSliceHash(t *testing.T) {
	t.Parallel()
	h := goodHash()
	m := Manifest{
		Prompt:        ArtifactRef{Kind: ArtifactPrompt, Hash: h},
		ModelPolicy:   ArtifactRef{Kind: ArtifactModelPolicy, Hash: h},
		RuntimePolicy: ArtifactRef{Kind: ArtifactRuntimePolicy, Hash: h},
		Context:       ArtifactRef{Kind: ArtifactContext, Hash: h},
		Memory:        ArtifactRef{Kind: ArtifactMemory, Hash: h},
		Tools:         []ArtifactRef{{Kind: ArtifactTool, Hash: h}, {Kind: ArtifactTool, Hash: h}},
	}
	if err := m.Validate(); err == nil {
		t.Fatalf("expected duplicate-hash error")
	}
}

// goodHash returns a known-good 64-character lowercase hex string.
// It is intentionally not all-zero so length and character-class
// checks are exercised against a realistic input.
func goodHash() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
