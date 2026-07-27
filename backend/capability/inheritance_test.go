package capability

import (
	"errors"
	"testing"
)

// fakeResolver is a tiny in-memory InheritanceResolver for
// tests. Add the parents via the map; missing keys return an
// error to exercise the parent-missing branch.
type fakeResolver struct {
	versions map[string]*Version
}

func (f *fakeResolver) GetVersion(id string) (*Version, error) {
	v, ok := f.versions[id]
	if !ok {
		return nil, errors.New("fakeResolver: not found: " + id)
	}
	return v, nil
}

func TestResolveManifestNoParentsReturnsSameManifest(t *testing.T) {
	t.Parallel()
	v := &Version{ID: "v1", CapabilityID: "c1", Manifest: minimalManifest()}
	resolver := &fakeResolver{versions: map[string]*Version{}}
	got, err := ResolveManifest(v, resolver)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Prompt.Hash != minimalManifest().Prompt.Hash {
		t.Errorf("expected unchanged prompt hash")
	}
}

func TestResolveManifestInheritsParentsArtifacts(t *testing.T) {
	t.Parallel()
	parentPrompt := ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()}
	parentModel := ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()}
	parent := &Version{
		ID: "parent-v1", CapabilityID: "parent",
		Manifest: Manifest{Prompt: parentPrompt, ModelPolicy: parentModel},
	}
	childPrompt := ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()}
	childModel := ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()}
	child := &Version{
		ID: "child-v1", CapabilityID: "child",
		Parents:  []string{"parent-v1"},
		Manifest: Manifest{Prompt: childPrompt, ModelPolicy: childModel},
	}
	resolver := &fakeResolver{versions: map[string]*Version{
		"parent-v1": parent,
	}}
	got, err := ResolveManifest(child, resolver)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Child overrides parent.
	if got.Prompt.Hash != childPrompt.Hash {
		t.Errorf("prompt: child override failed")
	}
	if got.ModelPolicy.Hash != childModel.Hash {
		t.Errorf("model policy: child override failed")
	}
}

func TestResolveManifestFallsBackToParent(t *testing.T) {
	t.Parallel()
	parentTool := ArtifactRef{Kind: ArtifactTool, Hash: goodHash()}
	parentGuard := ArtifactRef{Kind: ArtifactGuardrail, Hash: goodHash()}
	parent := &Version{
		ID: "parent-v1", CapabilityID: "parent",
		Manifest: Manifest{
			Prompt:      ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
			ModelPolicy: ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()},
			Tools:       []ArtifactRef{parentTool},
			Guardrails:  []ArtifactRef{parentGuard},
		},
	}
	// Child has only the three required artifacts; the tools
	// and guardrails should be inherited from the parent.
	child := &Version{
		ID: "child-v1", CapabilityID: "child",
		Parents:  []string{"parent-v1"},
		Manifest: minimalManifest(),
	}
	resolver := &fakeResolver{versions: map[string]*Version{
		"parent-v1": parent,
	}}
	got, err := ResolveManifest(child, resolver)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got.Tools) != 1 || got.Tools[0].Hash != parentTool.Hash {
		t.Errorf("tool inheritance failed: got %v", got.Tools)
	}
	if len(got.Guardrails) != 1 || got.Guardrails[0].Hash != parentGuard.Hash {
		t.Errorf("guardrail inheritance failed: got %v", got.Guardrails)
	}
}

func TestResolveManifestDetectsCycle(t *testing.T) {
	t.Parallel()
	a := &Version{ID: "a", CapabilityID: "A", Parents: []string{"b"}, Manifest: minimalManifest()}
	b := &Version{ID: "b", CapabilityID: "B", Parents: []string{"a"}, Manifest: minimalManifest()}
	resolver := &fakeResolver{versions: map[string]*Version{
		"a": a, "b": b,
	}}
	_, err := ResolveManifest(a, resolver)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cycle *ErrInheritanceCycle
	if !errors.As(err, &cycle) {
		t.Errorf("expected ErrInheritanceCycle, got %T %v", err, err)
	}
}

func TestResolveManifestDepthLimit(t *testing.T) {
	t.Parallel()
	// Build a chain of length MaxInheritanceDepth+1.
	chain := make([]*Version, 0, MaxInheritanceDepth+2)
	for i := 0; i < MaxInheritanceDepth+2; i++ {
		v := &Version{
			ID:           "v" + itoa(i),
			CapabilityID: "c" + itoa(i),
			Manifest:     minimalManifest(),
		}
		if i > 0 {
			v.Parents = []string{"v" + itoa(i-1)}
		}
		chain = append(chain, v)
	}
	resolver := &fakeResolver{versions: map[string]*Version{}}
	for _, v := range chain {
		resolver.versions[v.ID] = v
	}
	_, err := ResolveManifest(chain[len(chain)-1], resolver)
	if !errors.Is(err, ErrInheritanceTooDeep) {
		t.Errorf("expected ErrInheritanceTooDeep, got %v", err)
	}
}

func TestResolveManifestParentMissing(t *testing.T) {
	t.Parallel()
	child := &Version{
		ID: "child-v1", CapabilityID: "child",
		Parents:  []string{"nonexistent"},
		Manifest: minimalManifest(),
	}
	resolver := &fakeResolver{versions: map[string]*Version{}}
	_, err := ResolveManifest(child, resolver)
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
}

func TestMergeManifestsOverrideWins(t *testing.T) {
	t.Parallel()
	base := Manifest{
		Prompt:      ArtifactRef{Kind: ArtifactPrompt, Hash: "aaaa"},
		ModelPolicy: ArtifactRef{Kind: ArtifactModelPolicy, Hash: "bbbb"},
		Tools:       []ArtifactRef{{Kind: ArtifactTool, Hash: "t1"}},
		Guardrails:  []ArtifactRef{{Kind: ArtifactGuardrail, Hash: "g1"}},
	}
	override := Manifest{
		Prompt:     ArtifactRef{Kind: ArtifactPrompt, Hash: "cccc"},
		Tools:      []ArtifactRef{{Kind: ArtifactTool, Hash: "t2"}},
		Guardrails: []ArtifactRef{{Kind: ArtifactGuardrail, Hash: "g1"}},
	}
	got := mergeManifests(base, override)
	if got.Prompt.Hash != "cccc" {
		t.Errorf("prompt override failed")
	}
	if got.ModelPolicy.Hash != "bbbb" {
		t.Errorf("model policy should inherit base")
	}
	if len(got.Tools) != 2 {
		t.Errorf("tools union expected 2, got %d", len(got.Tools))
	}
	if len(got.Guardrails) != 1 {
		t.Errorf("guardrails should dedupe to 1, got %d", len(got.Guardrails))
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func minimalManifest() Manifest {
	return Manifest{
		Prompt:        ArtifactRef{Kind: ArtifactPrompt, Hash: goodHash()},
		ModelPolicy:   ArtifactRef{Kind: ArtifactModelPolicy, Hash: goodHash()},
		RuntimePolicy: ArtifactRef{Kind: ArtifactRuntimePolicy, Hash: goodHash()},
	}
}

func goodHash() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
