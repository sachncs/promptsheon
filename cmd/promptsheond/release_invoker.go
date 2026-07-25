package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/store"
)

// apiReleaseInvoker adapts the daemon's invoke.Invoker into the
// harness.ReleaseInvoker contract. The harness eval loop calls
// this once per dataset case; the adapter resolves the Release's
// manifest through the shared release.Resolver and forwards
// Provider / Model / ModelRevision to the invoke path so eval
// cases route through the same provider wiring as the live
// /releases/{id}/invoke route.
type apiReleaseInvoker struct {
	db       *store.SQLite
	inv      *invoke.Invoker
	svc      *release.Service
	resolver *release.Resolver
}

func (r *apiReleaseInvoker) Invoke(ctx context.Context, releaseID string, inputs map[string]any) (json.RawMessage, error) {
	rel, err := r.svc.Get(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if rel.Status != release.StatusActive {
		return nil, fmt.Errorf("release %s is not active", releaseID)
	}
	input, err := json.Marshal(inputs)
	if err != nil {
		return nil, err
	}
	// ponytail: Resolver was missing here, so Provider/Model were
	// blank and the executor's caller refused every eval case with
	// ErrProviderMissing. Thread the resolver through to mirror
	// invokeOneWithManifest's wiring.
	plan, err := r.resolver.Resolve(ctx, rel.ID)
	if err != nil {
		return nil, err
	}
	rec, err := r.inv.Invoke(ctx, executor.InvokeRequest{
		ReleaseID:     rel.ID,
		ManifestHash:  manifestHash(rel.Manifest),
		InputHash:     inputHash(input),
		Input:         input,
		Provider:      plan.Provider,
		Model:         plan.Model,
		ModelRevision: modelRevision(plan.Model, plan.Provider),
		SystemPrompt:  plan.Prompt,
	})
	if err != nil {
		return nil, err
	}
	// ponytail: previously coerced empty output to `json.RawMessage(""")`
	// (an empty JSON string), which is lossy: the eval scorer would
	// see a present-but-empty string instead of absent output. Return
	// the zero value so downstream consumers (DB, scorer) can tell
	// "no output" from "empty string output".
	if len(rec.Output) == 0 {
		return nil, nil
	}
	return rec.Output, nil
}

// modelRevision mirrors internal/api.handlers_capabilities.modelRevision
// without dragging the api package into cmd/promptsheond. Format is
// stable: YYYY-MM-DD:model:provider.
func modelRevision(model, provider string) string {
	return time.Now().UTC().Format("2006-01-02") + ":" + model + ":" + provider
}

func manifestHash(m interface{}) string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func inputHash(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
