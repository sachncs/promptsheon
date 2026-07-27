package capability

// ManifestDiff is the structural difference between two
// Manifests. Lists the artifact references that were added,
// removed, or whose hash changed between from and to.
type ManifestDiff struct {
	FromVersion    int             `json:"from_version"`
	ToVersion      int             `json:"to_version"`
	Added          []ArtifactRef   `json:"added"`
	Removed        []ArtifactRef   `json:"removed"`
	ChangedPrompts []ChangedPrompt `json:"changed,omitempty"`
}

// ChangedPrompt records an artifact whose hash changed between
// from and to. The pair is (kind, oldHash, newHash).
type ChangedPrompt struct {
	Kind    ArtifactKind `json:"kind"`
	OldHash string       `json:"old_hash"`
	NewHash string       `json:"new_hash"`
}

// DiffManifests returns the structural diff between two
// Manifests. The comparison is by ArtifactRef identity (the
// (Kind, Hash) tuple); an artifact whose Kind matches in both
// Manifests but whose Hash differs is reported as Changed.
//
// Order of operations is intentional: walk the "from" manifest
// first to surface removals and changes, then walk the "to"
// manifest to surface additions. The result is deterministic
// for the same input pair.
func DiffManifests(from, to Manifest) ManifestDiff {
	d := ManifestDiff{}
	fromIdx := manifestIndex(from)
	toIdx := manifestIndex(to)
	// Removals and changes: every ref in `from` that isn't in
	// `to` with the same hash.
	for kind, refs := range fromIdx {
		for _, r := range refs {
			tos, ok := toIdx[kind]
			if !ok {
				d.Removed = append(d.Removed, r)
				continue
			}
			found := false
			for _, tr := range tos {
				if tr.Hash == r.Hash {
					found = true
					break
				}
			}
			if !found {
				// The Kind has a matching entry but with a
				// different hash — it's a change, not a
				// removal. Surface the (old, new) pair.
				if len(tos) > 0 {
					d.ChangedPrompts = append(d.ChangedPrompts, ChangedPrompt{
						Kind:    kind,
						OldHash: r.Hash,
						NewHash: tos[0].Hash,
					})
				} else {
					d.Removed = append(d.Removed, r)
				}
			}
		}
	}
	// Additions: every ref in `to` not in `from`.
	for kind, refs := range toIdx {
		froms := fromIdx[kind]
		for _, r := range refs {
			found := false
			for _, fr := range froms {
				if fr.Hash == r.Hash {
					found = true
					break
				}
			}
			if !found {
				d.Added = append(d.Added, r)
			}
		}
	}
	return d
}

// manifestIndex groups a Manifest's artifact refs by Kind
// for fast lookup. The required single refs (Prompt,
// ModelPolicy, RuntimePolicy) are returned as single-element
// slices so the loop above can treat them uniformly.
func manifestIndex(m Manifest) map[ArtifactKind][]ArtifactRef {
	idx := map[ArtifactKind][]ArtifactRef{
		ArtifactPrompt:        {m.Prompt},
		ArtifactModelPolicy:   {m.ModelPolicy},
		ArtifactRuntimePolicy: {m.RuntimePolicy},
	}
	if m.Context.Hash != "" {
		idx[ArtifactContext] = []ArtifactRef{m.Context}
	}
	if m.Memory.Hash != "" {
		idx[ArtifactMemory] = []ArtifactRef{m.Memory}
	}
	idx[ArtifactGuardrail] = append([]ArtifactRef(nil), m.Guardrails...)
	idx[ArtifactTool] = append([]ArtifactRef(nil), m.Tools...)
	idx[ArtifactMCPServer] = append([]ArtifactRef(nil), m.MCPServers...)
	return idx
}
