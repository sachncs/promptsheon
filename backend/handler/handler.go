// Package handler extracts shared HTTP handler helpers from the API
// layer into a testable package with explicit dependency injection.
//
// Each function receives a *Deps instead of reaching into a *Server
// struct, making it easy to wire in tests without constructing a full
// API server.
package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/backend/auth"
	"github.com/sachncs/promptsheon/backend/capability"
	"github.com/sachncs/promptsheon/backend/executor"
	"github.com/sachncs/promptsheon/backend/invoke"
	"github.com/sachncs/promptsheon/backend/reasoning"
	"github.com/sachncs/promptsheon/backend/release"
	"github.com/sachncs/promptsheon/backend/settings"
	"github.com/sachncs/promptsheon/backend/store"
)

// Deps holds the dependencies required by handler helper functions.
type Deps struct {
	DB                *store.Repositories
	Authn             *auth.Authenticator
	RequireAuth       bool
	Invoker           *invoke.Invoker
	ReleaseResolver   *release.Resolver
	SettingsNotif     *settings.Notifier
	SettingsReplicaID string
	CapRepo           capability.Repository
}

// AuditFunc is the signature for audit callbacks.
type AuditFunc func(ctx context.Context, action, resource string, details map[string]any)

// ReasoningCatalog returns the catalog the reasoning Compiler consumes.
func ReasoningCatalog(ctx context.Context, d *Deps) ([]reasoning.CapabilityDescriptor, error) {
	if d.CapRepo == nil {
		return nil, nil
	}
	caps, err := d.CapRepo.ListCapabilities(ctx, "")
	if err != nil || len(caps) == 0 {
		return nil, err
	}
	out := make([]reasoning.CapabilityDescriptor, 0, len(caps))
	for _, c := range caps {
		rep, _ := d.CapRepo.GetCapabilityReputation(ctx, c.ID)
		out = append(out, reasoning.CapabilityDescriptor{
			ID:         c.ID,
			Name:       c.Name,
			Tags:       c.Tags,
			TrustScore: rep.TrustScore,
			CostUSD:    0.001,
			LatencyMS:  500,
			Outputs:    []string{"result"},
		})
	}
	return out, nil
}

// SettingsResolver builds a per-request view of the settings layer.
func SettingsResolver(d *Deps) (*settings.Resolver, error) {
	if d.SettingsNotif == nil {
		return nil, errors.New("settings: notifier not configured")
	}
	if d.SettingsReplicaID == "" {
		return nil, errors.New("settings: replica id not configured")
	}
	return settings.NewResolver(d.DB, d.SettingsNotif, nil, d.SettingsReplicaID), nil
}

// ResolveRelease builds a ResolvedInvocation for a release.
// Returns (nil, nil) when no Resolver is configured.
func ResolveRelease(ctx context.Context, d *Deps, rel *release.Release) (*release.ResolvedInvocation, error) {
	if d.ReleaseResolver == nil {
		return nil, nil
	}
	return d.ReleaseResolver.Resolve(ctx, rel.ID)
}

// AuthenticateRequest runs the configured authenticator on the request
// and attaches the resulting user to the context. Returns the original
// request unchanged when auth is disabled.
func AuthenticateRequest(r *http.Request, d *Deps) (*http.Request, *auth.User, error) {
	if !d.RequireAuth || d.Authn == nil {
		return r, nil, nil
	}
	user, err := d.Authn.Authenticate(r)
	if err != nil {
		return r, nil, err
	}
	return r.WithContext(auth.WithUserContext(r.Context(), user)), user, nil
}

// InvokeOne invokes a single capability version. Requires Invoker to
// be set; a missing invoker returns a clear error.
func InvokeOne(ctx context.Context, d *Deps, req executor.InvokeRequest) (*executor.ExecutionRecord, error, time.Duration) {
	if d.Invoker == nil {
		return nil, errors.New("handler: invoke.Invoker not wired"), 0
	}
	start := time.Now()
	rec, err := d.Invoker.Invoke(ctx, req)
	return &rec, err, time.Since(start)
}
