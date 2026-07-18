package api

import (
	"context"
	"encoding/json"

	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/release"
)

// releaseInvoker adapts the existing invoke.Invoker path (used by
// handleInvokeRelease) to the harness.ReleaseInvoker interface
// (used by harness.EvalRunner). The adapter bypasses the
// HTTP-handler-specific invokeOneWithManifest and calls the
// invoke.Invoker directly with a minimal InvokeRequest, since
// ReleaseInvoker only needs the LLM output and the harness runs
// outside any HTTP context.
type releaseInvoker struct {
	s *Server
}

// Invoke fetches the Release, looks up the Capability Version,
// then runs the invoke.Invoker and returns the recorded Output.
func (r *releaseInvoker) Invoke(ctx context.Context, releaseID string, inputs map[string]any) (json.RawMessage, error) {
	rel, err := r.s.releaseSvc.Get(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if rel.Status != release.StatusActive {
		return nil, errReleaseNotActive
	}
	input, err := marshalNoArgs(inputs)
	if err != nil {
		return nil, err
	}
	manifestHash, _ := computeManifestHash(rel.Manifest)
	rec, err := r.s.invoker.Invoke(ctx, executor.InvokeRequest{
		ReleaseID:    rel.ID,
		ManifestHash: manifestHash,
		InputHash:    inputHash(input),
		Input:        input,
	})
	if err != nil {
		return nil, err
	}
	if len(rec.Output) == 0 {
		return json.RawMessage(`""`), nil
	}
	return rec.Output, nil
}

// errReleaseNotActive is returned when a Release that's not yet
// Active is the target of an eval Invoke.
var errReleaseNotActive = errNotActive{}

type errNotActive struct{}

func (errNotActive) Error() string { return "release is not active" }
