// Package main: adapters between the selfevolve package
// and the daemon's real collaborators. The selfevolve
// package is otherwise self-contained.
package main

import (
	"github.com/sachncs/promptsheon/backend/errs"
	"context"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/backend/approval"
	"github.com/sachncs/promptsheon/backend/harness"
	"github.com/sachncs/promptsheon/backend/release"
	"github.com/sachncs/promptsheon/backend/selfevolve"
	"github.com/sachncs/promptsheon/backend/store"
)

// evolverRepoAdapter wraps *store.SQLite and satisfies
// selfevolve.Repository. The harness.Repository and
// capability.Repository methods are satisfied by the
// embedded *store.SQLite; the evolver-only extensions
// (env-filtered ActiveReleaseID, evolver-typed
// ReleaseRecord, etc.) are implemented here.
type evolverRepoAdapter struct {
	*store.SQLite
}

func newEvolverRepoAdapter(s *store.SQLite) *evolverRepoAdapter {
	return &evolverRepoAdapter{SQLite: s}
}

// ActiveReleaseID satisfies selfevolve.Repository. The
// harness.Repository's GetActiveReleaseID takes only
// capabilityID; the evolver's variant also takes env.
func (a *evolverRepoAdapter) ActiveReleaseID(ctx context.Context, capabilityID, env string) (string, error) {
	return a.GetActiveReleaseIDInEnv(ctx, capabilityID, env)
}

// GetRelease returns the evolver's minimal ReleaseRecord
// rather than the full release.Release struct.
func (a *evolverRepoAdapter) GetRelease(ctx context.Context, id string) (*selfevolve.ReleaseRecord, error) {
	rel, err := a.SQLite.GetRelease(ctx, id)
	if err != nil || rel == nil {
		return nil, err
	}
	return &selfevolve.ReleaseRecord{
		ID:                rel.ID,
		CapabilityID:      rel.CapabilityID,
		CapabilityVersion: rel.CapabilityVersion,
		Manifest:          rel.Manifest,
		Environment:       string(rel.Environment),
		Status:            string(rel.Status),
		CreatedBy:         rel.CreatedBy,
		CreatedAt:         rel.CreatedAt,
	}, nil
}

// LastEvalRun returns the most recent EvalRun for a
// release. Used to read the latest score without
// scanning the table.
func (a *evolverRepoAdapter) LastEvalRun(ctx context.Context, releaseID string) (*harness.EvalRun, error) {
	return a.LastEvalRunForRelease(ctx, releaseID)
}

// UpdateReleaseStatus marks a release as superseded or
// rolled back. Used when the evolver aborts a cycle.
func (a *evolverRepoAdapter) UpdateReleaseStatus(ctx context.Context, releaseID, status string) error {
	rel, err := a.SQLite.GetRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if rel == nil {
		return errs.ErrStoreNotFound
	}
	rel.Status = release.Status(status)
	return a.SQLite.UpdateRelease(ctx, rel)
}

// CreateRelease creates a Pending release. The evolver
// then calls SelfActivate which drives the Status
// transition.
func (a *evolverRepoAdapter) CreateRelease(ctx context.Context, rec selfevolve.ReleaseRecord) error {
	r := &release.Release{
		ID:                rec.ID,
		CapabilityID:      rec.CapabilityID,
		CapabilityVersion: rec.CapabilityVersion,
		Manifest:          rec.Manifest,
		Environment:       release.Environment(rec.Environment),
		Status:            release.Status(rec.Status),
		CreatedBy:         rec.CreatedBy,
		CreatedAt:         rec.CreatedAt,
	}
	return a.SQLite.CreateRelease(ctx, r)
}

// evolverActivatorAdapter exposes release.Service.SelfActivate
// as a selfevolve.ReleaseActivator. It also writes the
// synthetic Approve vote so the SelfApprovePolicy has
// something to evaluate.
type evolverActivatorAdapter struct {
	svc          *release.Service
	selfApprover string
	repo         approval.Repository
}

func (a *evolverActivatorAdapter) SelfActivate(ctx context.Context, releaseID string) error {
	if a.svc == nil {
		return fmt.Errorf("evolver: activator not wired")
	}
	if a.selfApprover == "" {
		return fmt.Errorf("evolver: activator not configured (self-approver identity missing)")
	}
	// SelfApprovePolicy evaluates the existing approval
	// row's votes. The regular Activate path expects a
	// human to have voted first; for self-evolve we have
	// no human, so we write the synthetic Approve vote
	// here. CreateApproval is a no-op for the existing row
	// so retries are safe.
	now := time.Now().UTC()
	if err := a.repo.CreateApproval(ctx, &approval.Approval{
		ReleaseID: releaseID,
		Votes: []approval.Vote{{
			Identity:  a.selfApprover,
			Decision:  approval.Approve,
			Timestamp: now,
		}},
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("evolver: write self-approval: %w", err)
	}
	_, err := a.svc.SelfActivate(ctx, releaseID)
	return err
}

// evolverAuditorAdapter exposes the daemon's audit
// recorder as a selfevolve.Auditor.
type evolverAuditorAdapter struct {
	auditor AuditWriter
}

// AuditWriter is the minimal audit interface the
// evolver needs.
type AuditWriter interface {
	Audit(ctx context.Context, action, target string, detail map[string]any)
}

func (a *evolverAuditorAdapter) Audit(ctx context.Context, action, target string, detail map[string]any) {
	a.auditor.Audit(ctx, action, target, detail)
}
