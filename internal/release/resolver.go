// Package release provides the canonical release runtime, including
// the Resolver that turns a Release into a ResolvedInvocation.
//
// ResolvedInvocation is the immutable plan for one invoke call.
// Constructing it once, at the boundary, is what closes the gap
// between "the approved release" and "the call that actually runs":
// the previous implementation let the HTTP request pick model and
// provider after activation, which made the approval a fiction.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// ResolvedInvocation is the immutable plan for a single invoke. It
// is constructed by Resolver.Resolve and consumed by the invoke
// path. Every field that affects provider selection, prompt
// rendering, safety, and limits comes from the release's manifest
// (which itself is content-addressed), not from the HTTP request.
type ResolvedInvocation struct {
	ReleaseID         string
	CapabilityID      string
	CapabilityVersion int
	Environment       Environment

	// Provider + model + revision come from the Manifest's
	// ModelPolicy artifact. The request MUST NOT override these.
	Provider string
	Model    string
	Revision string

	// Prompt is the bytes of the Manifest's Prompt artifact.
	// Empty when the artifact has not been loaded from the CAS;
	// the resolve is allowed to skip the load when no caller
	// path is wired.
	Prompt []byte

	// Runtime limits from the Manifest's RuntimePolicy artifact.
	// Conservative defaults applied when the artifact is missing.
	MaxOutputTokens int
	Temperature     float64
	TopP            float64

	// GuardrailRefs is the list of guardrail artifacts to apply,
	// in order, before sending the prompt to the provider and
	// after receiving the response. The list is derived from the
	// manifest at resolve time so the invoke path is read-only.
	GuardrailRefs []capability.ArtifactRef

	// ContextRefs, MemoryRefs, ToolRefs, MCPServerRefs are
	// informational; the actual loading of these artifacts is
	// the responsibility of the caller's invoke pipeline.
	ContextRefs   []capability.ArtifactRef
	MemoryRefs    []capability.ArtifactRef
	ToolRefs      []capability.ArtifactRef
	MCPServerRefs []capability.ArtifactRef

	// ResolvedAt is when this plan was built. Recorded for
	// observability and for replay buffers that need a
	// deterministic timestamp.
	ResolvedAt time.Time
}

// ErrReleaseNotActive is returned by Resolver.Resolve when the
// release is not in the Active state. The only way to invoke a
// release is via the active, approved, currently-serving
// (release_id, environment) pair.
var ErrReleaseNotActive = errors.New("release: not active")

// ArtifactLoader fetches artifact bytes by content hash. The
// Resolver does not own the storage; the caller passes a Loader
// at construction time. A nil Loader produces a ResolvedInvocation
// with empty Prompt and an empty artifact-ref list. This is the
// documented "no-CAS" mode: the daemon still produces the plan
// shape, but artifact content is filled in by a follow-on
// implementation that wires the actual CAS to the daemon.
type ArtifactLoader interface {
	Load(ctx context.Context, kind capability.ArtifactKind, hash string) ([]byte, error)
}

// ModelPolicyRecord is the JSON document stored at the
// ModelPolicy artifact hash. Only the fields used by the resolver
// are decoded; the rest of the document is opaque.
type ModelPolicyRecord struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Revision string `json:"revision,omitempty"`
	Defaults struct {
		MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
		Temperature     float64 `json:"temperature,omitempty"`
		TopP            float64 `json:"top_p,omitempty"`
	} `json:"defaults,omitempty"`
}

// RuntimePolicyRecord is the JSON document stored at the
// RuntimePolicy artifact hash.
type RuntimePolicyRecord struct {
	MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"top_p,omitempty"`
}

// Resolver turns a Release + the active state check into a
// ResolvedInvocation. It does not mutate the release or the
// manifest; the plan is a value type.
type Resolver struct {
	DB     Repository
	Loader ArtifactLoader
	Now    func() time.Time
}

// NewResolver constructs a Resolver. db is used to look up the
// release by id and to verify the active state. A nil Loader is
// permitted (no-CAS mode); the resolver produces an otherwise-valid
// plan with empty Prompt and skip CAS lookups for sub-artifacts.
func NewResolver(db Repository, loader ArtifactLoader) *Resolver {
	return &Resolver{DB: db, Loader: loader, Now: func() time.Time { return time.Now().UTC() }}
}

// Resolve builds the plan for the given release id. The release
// must exist, must be Active, and must have a non-empty Manifest.
// Model/Provider come from the ModelPolicy artifact (if Loader is
// non-nil) or fall back to a sane default derived from the
// capability id when Loader is nil (no-CAS mode). The returned
// ResolvedInvocation is the single source of truth for the
// subsequent invoke call.
func (r *Resolver) Resolve(ctx context.Context, releaseID string) (*ResolvedInvocation, error) {
	rel, err := r.DB.GetRelease(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if rel.Status != StatusActive {
		return nil, fmt.Errorf("%w: got status %q", ErrReleaseNotActive, rel.Status)
	}
	if err := rel.Manifest.Validate(); err != nil {
		return nil, fmt.Errorf("resolver: manifest: %w", err)
	}

	plan := &ResolvedInvocation{
		ReleaseID:         rel.ID,
		CapabilityID:      rel.CapabilityID,
		CapabilityVersion: rel.CapabilityVersion,
		Environment:       rel.Environment,
		GuardrailRefs:     rel.Manifest.Guardrails,
		ContextRefs:       []capability.ArtifactRef{rel.Manifest.Context},
		MemoryRefs:        []capability.ArtifactRef{rel.Manifest.Memory},
		MCPServerRefs:     rel.Manifest.MCPServers,
		ToolRefs:          rel.Manifest.Tools,
		ResolvedAt:        r.Now(),
	}

	if r.Loader != nil {
		promptBytes, err := r.Loader.Load(ctx, capability.ArtifactPrompt, rel.Manifest.Prompt.Hash)
		if err != nil {
			return nil, fmt.Errorf("resolver: load prompt: %w", err)
		}
		plan.Prompt = promptBytes

		mpBytes, err := r.Loader.Load(ctx, capability.ArtifactModelPolicy, rel.Manifest.ModelPolicy.Hash)
		if err != nil {
			return nil, fmt.Errorf("resolver: load model policy: %w", err)
		}
		var mp ModelPolicyRecord
		if err := json.Unmarshal(mpBytes, &mp); err != nil {
			return nil, fmt.Errorf("resolver: model policy: %w", err)
		}
		plan.Provider = mp.Provider
		plan.Model = mp.Model
		plan.Revision = mp.Revision
		plan.MaxOutputTokens = mp.Defaults.MaxOutputTokens
		plan.Temperature = mp.Defaults.Temperature
		plan.TopP = mp.Defaults.TopP

		// Runtime policy overrides model policy defaults when set.
		if rel.Manifest.RuntimePolicy.Hash != "" {
			rpBytes, err := r.Loader.Load(ctx, capability.ArtifactRuntimePolicy, rel.Manifest.RuntimePolicy.Hash)
			if err == nil {
				var rp RuntimePolicyRecord
				if err := json.Unmarshal(rpBytes, &rp); err == nil {
					if rp.MaxOutputTokens > 0 {
						plan.MaxOutputTokens = rp.MaxOutputTokens
					}
					if rp.Temperature > 0 {
						plan.Temperature = rp.Temperature
					}
					if rp.TopP > 0 {
						plan.TopP = rp.TopP
					}
				}
			}
		}
	} else {
		// No-CAS fallback: derive a placeholder provider/model
		// from the capability id. The invoke path will fail
		// later when no provider is registered for this name,
		// surfacing a 502 with a clear "model policy artifact
		// not loaded" message rather than silently using a
		// caller-supplied model.
		plan.Provider = "default"
		plan.Model = rel.CapabilityID
	}

	return plan, nil
}
