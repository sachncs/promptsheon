package capability

import (
	"github.com/sachncs/promptsheon/errf"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// MaxInheritanceDepth bounds the inheritance chain length.
// A chain longer than this triggers an error at Resolve time;
// the cap protects against accidental cycles in the parent
// graph and against pathological manifests.
const MaxInheritanceDepth = 8

// ErrInheritanceCycle is returned by Resolve when the parent
// graph contains a cycle (A inherits from B which inherits
// from A). The error wraps the offending ids for diagnostics.
type ErrInheritanceCycle struct {
	IDs []string
}

func (e *ErrInheritanceCycle) Error() string {
	return fmt.Sprintf("capability: inheritance cycle through %v", e.IDs)
}

// errs.ErrInheritanceTooDeep is returned when the parent chain
// exceeds MaxInheritanceDepth.

// InheritanceResolver resolves a Version's parent chain into
// the effective Manifest. The resolver is a thin abstraction
// over the Repository so tests can inject a fake.
type InheritanceResolver interface {
	// GetVersion returns the Version with the given id, or
	// ErrNotFound if absent.
	GetVersion(id string) (*Version, error)
}

// ResolveManifest walks the parent chain rooted at v and
// returns the merged Manifest. The merge rule is "child
// overrides parent": for each artifact kind, the child's
// value wins; if the child has no value for a kind, the
// parent's value is inherited.
//
// Cycles return ErrInheritanceCycle; chains longer than
// MaxInheritanceDepth return errs.ErrInheritanceTooDeep.
func ResolveManifest(v *Version, resolver InheritanceResolver) (Manifest, error) {
	if v == nil {
		return Manifest{}, errors.New("capability: nil version")
	}
	merged := v.Manifest
	if len(v.Parents) == 0 {
		return merged, nil
	}
	visited := map[string]bool{v.CapabilityID + "/" + versionID(v): true}
	chain := []*Version{v}
	for _, parentID := range v.Parents {
		var err error
		merged, err = resolveParent(parentID, resolver, merged, visited, chain, 1)
		if err != nil {
			return Manifest{}, err
		}
	}
	return merged, nil
}

// resolveParent recurses into one parent. The chain slice
// records the visit stack for diagnostics.
func resolveParent(parentID string, resolver InheritanceResolver, base Manifest, visited map[string]bool, chain []*Version, depth int) (Manifest, error) {
	if depth > MaxInheritanceDepth {
		return Manifest{}, errs.ErrInheritanceTooDeep
	}
	parent, err := resolver.GetVersion(parentID)
	if err != nil {
		return Manifest{}, errf.Errorf("capability: load parent %s: %w", parentID, err)
	}
	key := parent.CapabilityID + "/" + versionID(parent)
	if visited[key] {
		cycle := make([]string, 0, len(chain)+1)
		for _, v := range chain {
			cycle = append(cycle, v.CapabilityID+"/"+versionID(v))
		}
		cycle = append(cycle, key)
		return Manifest{}, &ErrInheritanceCycle{IDs: cycle}
	}
	visited[key] = true
	chain = append(chain, parent)
	defer func() {
		delete(visited, key)
		chain = chain[:len(chain)-1]
	}()
	// Merge: parent first, then child (this Version's Manifest)
	// overrides. The order matters when both define the same
	// artifact kind.
	merged := mergeManifests(parent.Manifest, base)
	for _, grandParentID := range parent.Parents {
		merged, err = resolveParent(grandParentID, resolver, merged, visited, chain, depth+1)
		if err != nil {
			return Manifest{}, err
		}
	}
	return merged, nil
}

// mergeManifests returns a Manifest where `overrides` wins over
// `base` for every artifact kind. The merge is non-mutating.
func mergeManifests(base, overrides Manifest) Manifest {
	out := base
	if overrides.Prompt.Hash != "" {
		out.Prompt = overrides.Prompt
	}
	if overrides.ModelPolicy.Hash != "" {
		out.ModelPolicy = overrides.ModelPolicy
	}
	if overrides.RuntimePolicy.Hash != "" {
		out.RuntimePolicy = overrides.RuntimePolicy
	}
	if overrides.Context.Hash != "" {
		out.Context = overrides.Context
	}
	if overrides.Memory.Hash != "" {
		out.Memory = overrides.Memory
	}
	// Slice kinds: union, with override hash winning on
	// collisions.
	out.Guardrails = mergeArtifactSlice(base.Guardrails, overrides.Guardrails)
	out.Tools = mergeArtifactSlice(base.Tools, overrides.Tools)
	out.MCPServers = mergeArtifactSlice(base.MCPServers, overrides.MCPServers)
	return out
}

// mergeArtifactSlice returns the union of two slices. Hashes
// in overrides win over the same hash in base.
//
// The output is deterministically ordered: base entries appear
// first in base order, then override entries that introduced a
// new hash appear in override order. Without this guarantee,
// successive merges of the same inputs produced different hashes
// and broke content identity for the resulting manifest.
func mergeArtifactSlice(base, overrides []ArtifactRef) []ArtifactRef {
	seen := make(map[string]struct{}, len(base)+len(overrides))
	out := make([]ArtifactRef, 0, len(base)+len(overrides))
	for _, r := range base {
		if _, ok := seen[r.Hash]; ok {
			continue
		}
		seen[r.Hash] = struct{}{}
		out = append(out, r)
	}
	for _, r := range overrides {
		if _, ok := seen[r.Hash]; ok {
			continue
		}
		seen[r.Hash] = struct{}{}
		out = append(out, r)
	}
	return out
}

func versionID(v *Version) string {
	if v == nil {
		return ""
	}
	return v.ID
}
