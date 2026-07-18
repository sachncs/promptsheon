package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/store"
)

// apiReleaseInvoker adapts the daemon's invoke.Invoker into the
// harness.ReleaseInvoker contract. The harness eval loop calls
// this once per dataset case; the adapter looks up the Release
// and forwards to the existing invoke path so eval cases use the
// same provider wiring as the live /releases/{id}/invoke route.
type apiReleaseInvoker struct {
	db  *store.SQLite
	inv *invoke.Invoker
	svc *release.Service
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
	rec, err := r.inv.Invoke(ctx, executor.InvokeRequest{
		ReleaseID:    rel.ID,
		ManifestHash: manifestHash(rel.Manifest),
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
