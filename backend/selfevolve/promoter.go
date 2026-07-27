package selfevolve

import (
	"context"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
)

// Promoter turns a validated candidate prompt into an
// active Release in the target env. The flow:
//
//  1. read the active release's manifest (to copy the
//     model_policy / runtime_policy / context / memory
//     hashes)
//  2. write the new prompt to CAS
//  3. create a new capability.Version with the new
//     manifest
//  4. create a Pending Release in the target env
//  5. call ReleaseActivator.SelfActivate to flip it Active
type Promoter struct {
	Repo      Repository
	Loader    PromptLoader
	Activator ReleaseActivator
	Auditor   Auditor
	Now       func() time.Time
}

// Auditor writes audit rows. A nil Auditor is a
// programmer error: the evolver's safety contract is
// "every state change is auditable".
type Auditor interface {
	Audit(ctx context.Context, action, target string, detail map[string]any)
}

// NewPromoter constructs a Promoter. Now defaults to UTC. Returns
// an error when a required dependency is missing so callers fail
// fast rather than discovering a nil dereference mid-cycle.
func NewPromoter(repo Repository, loader PromptLoader, activator ReleaseActivator, auditor Auditor) (*Promoter, error) {
	if repo == nil {
		return nil, fmt.Errorf("selfevolve: promoter repo is required")
	}
	if loader == nil {
		return nil, fmt.Errorf("selfevolve: promoter loader is required")
	}
	if activator == nil {
		return nil, fmt.Errorf("selfevolve: promoter activator is required")
	}
	return &Promoter{
		Repo:      repo,
		Loader:    loader,
		Activator: activator,
		Auditor:   auditor,
		Now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

// PromoteResult is the outcome of Promote.
type PromoteResult struct {
	NewVersionID string
	NewReleaseID string
	OldReleaseID string
	NewHash      string
}

// Promote is the end-to-end promote. OldReleaseID is the
// currently-active release in the target env (recorded
// in the audit row for traceability); newPrompt is the
// validated revised prompt text.
func (p *Promoter) Promote(ctx context.Context, capabilityID, targetEnv string, oldReleaseID string, newPrompt string) (*PromoteResult, error) {
	if capabilityID == "" || targetEnv == "" {
		return nil, fmt.Errorf("selfevolve.promoter: missing capabilityID or targetEnv")
	}
	if newPrompt == "" {
		return nil, fmt.Errorf("selfevolve.promoter: empty new prompt")
	}
	oldManifest, err := p.loadActiveManifest(ctx, capabilityID, oldReleaseID)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: %w", err)
	}
	newHash, err := p.Loader.WritePrompt(ctx, newPrompt)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: write prompt: %w", err)
	}
	newManifest := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: newHash},
		ModelPolicy:   oldManifest.ModelPolicy,
		RuntimePolicy: oldManifest.RuntimePolicy,
		Context:       oldManifest.Context,
		Memory:        oldManifest.Memory,
	}
	manifestHash, err := capability.ComputeManifestHash(newManifest)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: compute manifest hash: %w", err)
	}
	nextVersion, err := p.nextVersionNumber(ctx, capabilityID)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: %w", err)
	}
	now := p.Now()
	versionID := generateID("v")
	version := &capability.Version{
		ID:           versionID,
		CapabilityID: capabilityID,
		Version:      nextVersion,
		Manifest:     newManifest,
		ManifestHash: manifestHash,
		CreatedAt:    now,
		CreatedBy:    "self_evolve",
	}
	if err := p.Repo.CreateVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: create version: %w", err)
	}
	releaseID := generateID("rel")
	rel := ReleaseRecord{
		ID:                releaseID,
		CapabilityID:      capabilityID,
		CapabilityVersion: nextVersion,
		Manifest:          newManifest,
		Environment:       targetEnv,
		Status:            "pending",
		CreatedBy:         "self_evolve",
		CreatedAt:         now,
	}
	if err := p.Repo.CreateRelease(ctx, rel); err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: create release: %w", err)
	}
	if p.Activator == nil {
		return nil, fmt.Errorf("selfevolve.promoter: activator not wired")
	}
	if err := p.Activator.SelfActivate(ctx, releaseID); err != nil {
		return nil, fmt.Errorf("selfevolve.promoter: self-activate: %w", err)
	}
	if p.Auditor != nil {
		p.Auditor.Audit(ctx, AuditPromote, "capability:"+capabilityID, map[string]any{
			"new_version_id":  versionID,
			"new_release_id":  releaseID,
			"old_release_id":  oldReleaseID,
			"new_prompt_hash": newHash,
			"target_env":      targetEnv,
			"manifest_hash":   manifestHash,
		})
	}
	return &PromoteResult{
		NewVersionID: versionID,
		NewReleaseID: releaseID,
		OldReleaseID: oldReleaseID,
		NewHash:      newHash,
	}, nil
}

// loadActiveManifest reads the manifest of the active
// release in targetEnv. If oldReleaseID is supplied, it
// looks that release up by id; otherwise it falls back
// to the capability's most recent version. The prompt
// hash from the loaded manifest is what the new manifest
// inherits the model_policy / runtime_policy / context /
// memory from.
func (p *Promoter) loadActiveManifest(ctx context.Context, capabilityID, oldReleaseID string) (capability.Manifest, error) {
	if oldReleaseID != "" {
		old, err := p.Repo.GetRelease(ctx, oldReleaseID)
		if err == nil && old != nil {
			oldVer, verr := p.Repo.GetVersionByNumber(ctx, capabilityID, old.CapabilityVersion)
			if verr == nil && oldVer != nil && oldVer.Manifest.Prompt.Hash != "" {
				return oldVer.Manifest, nil
			}
		}
	}
	// Fallback: walk back via the capability's versions
	// by probing. We treat "not found" as the loop
	// terminator so a fresh capability (no versions yet)
	// returns an error rather than looping forever.
	v := 1
	for {
		ver, err := p.Repo.GetVersionByNumber(ctx, capabilityID, v)
		if err != nil {
			return capability.Manifest{}, fmt.Errorf("selfevolve.promoter: no active manifest to copy model_policy / runtime_policy from (capabilityID=%q)", capabilityID)
		}
		if ver != nil && ver.Manifest.Prompt.Hash != "" {
			return ver.Manifest, nil
		}
		v++
		if v > 10000 {
			return capability.Manifest{}, fmt.Errorf("selfevolve.promoter: version probe exceeded 10000 for capability %s", capabilityID)
		}
	}
}

// nextVersionNumber returns the next integer version
// number for the capability: max(existing) + 1. We probe
// via GetVersionByNumber rather than a list call so the
// Repository surface stays narrow. O(versions) which is
// fine — capabilities rarely have more than a handful.
func (p *Promoter) nextVersionNumber(ctx context.Context, capabilityID string) (int, error) {
	v := 1
	for {
		_, err := p.Repo.GetVersionByNumber(ctx, capabilityID, v)
		if err != nil {
			return v, nil
		}
		v++
	}
}
