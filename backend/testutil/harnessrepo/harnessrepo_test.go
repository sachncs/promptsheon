package harnessrepo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/testutil/harnessrepo"
)

func TestDatasetCRUD(t *testing.T) {
	r := harnessrepo.New()
	ctx := context.Background()
	if err := r.CreateDataset(ctx, &harness.Dataset{ID: "d1", CapabilityID: "c1", Name: "ds"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Duplicate id rejected.
	if err := r.CreateDataset(ctx, &harness.Dataset{ID: "d1"}); err == nil {
		t.Errorf("expected duplicate id error")
	}
	got, err := r.GetDataset(ctx, "d1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CapabilityID != "c1" {
		t.Errorf("CapabilityID = %q, want c1", got.CapabilityID)
	}
	if _, err := r.GetDataset(ctx, "missing"); !errors.Is(err, harnessrepo.ErrNotFound) {
		t.Errorf("missing lookup: err = %v, want ErrNotFound", err)
	}
	// ListDatasetsForCapability filters by capability.
	if err := r.CreateDataset(ctx, &harness.Dataset{ID: "d2", CapabilityID: "c2"}); err != nil {
		t.Fatalf("Create d2: %v", err)
	}
	gotC1, err := r.ListDatasetsForCapability(ctx, "c1")
	if err != nil {
		t.Fatalf("List c1: %v", err)
	}
	if len(gotC1) != 1 || gotC1[0].ID != "d1" {
		t.Errorf("c1 list = %+v, want [d1]", gotC1)
	}
	if err := r.DeleteDataset(ctx, "d1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.GetDataset(ctx, "d1"); !errors.Is(err, harnessrepo.ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDatasetCases(t *testing.T) {
	r := harnessrepo.New()
	ctx := context.Background()
	if err := r.CreateDataset(ctx, &harness.Dataset{ID: "d1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cases := []harness.DatasetCase{
		{ID: "k0", DatasetID: "d1", Seq: 0},
		{ID: "k1", DatasetID: "d1", Seq: 1},
	}
	if err := r.UpsertDatasetCases(ctx, "d1", cases); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := r.ListDatasetCases(ctx, "d1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	// The fixture must return a COPY, not the slice header.
	got[0].Seq = 99
	got2, _ := r.ListDatasetCases(ctx, "d1")
	if got2[0].Seq == 99 {
		t.Errorf("List returned the same slice; mutation leaked")
	}
}

func TestPreconditionCRUD(t *testing.T) {
	r := harnessrepo.New()
	ctx := context.Background()
	if err := r.CreatePrecondition(ctx, &harness.Precondition{ID: "p1", CapabilityID: "c1", Name: "go-test", Command: "go test", TimeoutSec: 60}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.UpdatePrecondition(ctx, &harness.Precondition{ID: "p1", CapabilityID: "c1", Name: "go-test", Command: "go test -v", TimeoutSec: 120}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := r.GetPrecondition(ctx, "p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TimeoutSec != 120 {
		t.Errorf("TimeoutSec = %d, want 120", got.TimeoutSec)
	}
	// ListPreconditionsForCapability filters by capability.
	r.Preconds["p2"] = &harness.Precondition{ID: "p2", CapabilityID: "c2", Name: "lint"}
	list, err := r.ListPreconditionsForCapability(ctx, "c1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != "p1" {
		t.Errorf("c1 list = %+v, want [p1]", list)
	}
	if err := r.UpdatePrecondition(ctx, &harness.Precondition{ID: "missing"}); !errors.Is(err, harnessrepo.ErrNotFound) {
		t.Errorf("Update missing: err = %v, want ErrNotFound", err)
	}
	if err := r.DeletePrecondition(ctx, "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestEvalRunAndResults(t *testing.T) {
	r := harnessrepo.New()
	ctx := context.Background()
	if err := r.CreateEvalRun(ctx, &harness.EvalRun{ID: "r1", ReleaseID: "rel-1", DatasetID: "d1", Scorer: "contains"}); err != nil {
		t.Fatalf("CreateEvalRun: %v", err)
	}
	// Duplicate id.
	if err := r.CreateEvalRun(ctx, &harness.EvalRun{ID: "r1"}); err == nil {
		t.Errorf("expected duplicate run id error")
	}
	if err := r.UpdateEvalRun(ctx, &harness.EvalRun{ID: "r1", ReleaseID: "rel-1", DatasetID: "d1", Scorer: "regex"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := r.GetEvalRun(ctx, "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Scorer != "regex" {
		t.Errorf("Scorer = %q, want regex", got.Scorer)
	}
	// ListEvalRunsForRelease filters by release id.
	if err := r.CreateEvalRun(ctx, &harness.EvalRun{ID: "r2", ReleaseID: "rel-2", DatasetID: "d1", Scorer: "contains"}); err != nil {
		t.Fatalf("CreateEvalRun r2: %v", err)
	}
	list, err := r.ListEvalRunsForRelease(ctx, "rel-1")
	if err != nil {
		t.Fatalf("List rel-1: %v", err)
	}
	if len(list) != 1 || list[0].ID != "r1" {
		t.Errorf("rel-1 list = %+v, want [r1]", list)
	}
	if err := r.UpdateEvalRun(ctx, &harness.EvalRun{ID: "missing"}); !errors.Is(err, harnessrepo.ErrNotFound) {
		t.Errorf("Update missing: err = %v, want ErrNotFound", err)
	}
	// Results: both bulk and single insert paths.
	if err := r.CreateEvalResults(ctx, []harness.EvalResult{
		{ID: "res-0", RunID: "r1", CaseID: "k0", Seq: 0, Passed: true},
		{ID: "res-1", RunID: "r1", CaseID: "k1", Seq: 1, Passed: false},
	}); err != nil {
		t.Fatalf("CreateEvalResults: %v", err)
	}
	if err := r.CreateEvalResult(ctx, &harness.EvalResult{ID: "res-2", RunID: "r1", CaseID: "k2", Seq: 2, Passed: true}); err != nil {
		t.Fatalf("CreateEvalResult: %v", err)
	}
	results, err := r.ListEvalResultsForRun(ctx, "r1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len = %d, want 3", len(results))
	}
}

func TestGetActiveReleaseID(t *testing.T) {
	r := harnessrepo.New()
	ctx := context.Background()
	// Empty: no active release.
	got, err := r.GetActiveReleaseID(ctx, "c1")
	if err != nil {
		t.Fatalf("GetActiveReleaseID empty: %v", err)
	}
	if got != "" {
		t.Errorf("empty = %q, want \"\"", got)
	}
	// MemRepo does not implement release.Repository; insert
	// releases directly via the exported map (the fixture's
	// documented test surface).
	now := time.Now().UTC()
	r.Releases["rel-other"] = &release.Release{
		ID: "rel-other", CapabilityID: "c2", CapabilityVersion: 1,
		Environment: release.Environment("dev"),
		Status:      release.StatusActive,
		CreatedAt:   now, ActivatedAt: &now,
	}
	got, _ = r.GetActiveReleaseID(ctx, "c1")
	if got != "" {
		t.Errorf("after other-cap insert: c1 active = %q, want \"\"", got)
	}
	r.Releases["rel-c1"] = &release.Release{
		ID: "rel-c1", CapabilityID: "c1", CapabilityVersion: 1,
		Environment: release.Environment("dev"),
		Status:      release.StatusActive,
		CreatedAt:   now, ActivatedAt: &now,
	}
	got, _ = r.GetActiveReleaseID(ctx, "c1")
	if got != "rel-c1" {
		t.Errorf("c1 active = %q, want rel-c1", got)
	}
}
