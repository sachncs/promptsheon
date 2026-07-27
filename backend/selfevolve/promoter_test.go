package selfevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

type fakePromoterRepo struct {
	*fakeRepo
	releaseManifest capability.Manifest
}

func (f *fakePromoterRepo) GetRelease(ctx context.Context, id string) (*ReleaseRecord, error) {
	if id == "rel-old" {
		return &ReleaseRecord{
			ID:                "rel-old",
			CapabilityID:      "c1",
			CapabilityVersion: 1,
			Manifest:          f.releaseManifest,
			Environment:       "dev",
			Status:            "active",
		}, nil
	}
	return nil, nil
}

func (f *fakePromoterRepo) GetVersionByNumber(ctx context.Context, capID string, v int) (*capability.Version, error) {
	if v != 1 {
		return nil, fmt.Errorf("fakePromoterRepo: version %d not found", v)
	}
	return &capability.Version{
		ID:           "v-1",
		CapabilityID: capID,
		Version:      v,
		Manifest:     f.releaseManifest,
	}, nil
}

func newPromoterSetup(t *testing.T) (*fakePromoterRepo, *fakeLoader, *fakeActivator) {
	t.Helper()
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	activator := &fakeActivator{repo: inner}
	return &fakePromoterRepo{
		fakeRepo:        inner,
		releaseManifest: inner.versionsByCap["c1"][0].Manifest,
	}, loader, activator
}

func TestPromoter_NextVersionNumber_FirstVersion(t *testing.T) {
	repo, loader, activator := newPromoterSetup(t)
	p, perr := NewPromoter(repo, loader, activator, &fakeAuditor{})
	if perr != nil {
		t.Fatal(perr)
	}
	v, err := p.nextVersionNumber(context.Background(), "c1")
	if err != nil {
		t.Fatalf("nextVersionNumber: %v", err)
	}
	if v != 2 {
		t.Errorf("next version = %d, want 2", v)
	}
}

func TestPromoter_Promote_WritesCASAndCreatesRelease(t *testing.T) {
	repo, loader, activator := newPromoterSetup(t)
	p, perr := NewPromoter(repo, loader, activator, &fakeAuditor{})
	if perr != nil {
		t.Fatal(perr)
	}
	oldRelID := repo.activeReleaseID("c1", "dev")
	res, err := p.Promote(context.Background(), "c1", "dev", oldRelID, "new prompt text")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if res.NewVersionID == "" || res.NewReleaseID == "" {
		t.Errorf("missing ids: %+v", res)
	}
	// Old release must be superseded.
	old := repo.releaseByID(oldRelID)
	if old.Status != "superseded" {
		t.Errorf("old release status = %q, want superseded", old.Status)
	}
	// New release must be active.
	if repo.activeReleaseID("c1", "dev") != res.NewReleaseID {
		t.Errorf("new release not active")
	}
	// New prompt must be in the loader.
	_, err = loader.LoadPrompt(context.Background(), res.NewHash)
	if err != nil {
		t.Errorf("new prompt not in CAS: %v", err)
	}
	// Manifest hash of the new release must differ from the old.
	newRel := repo.releaseByID(res.NewReleaseID)
	if newRel.Manifest.Prompt.Hash == old.Manifest.Prompt.Hash {
		t.Errorf("new prompt hash equals old — revision had no effect")
	}
}

func TestPromoter_Promote_EmptyPrompt(t *testing.T) {
	repo, loader, activator := newPromoterSetup(t)
	p, perr := NewPromoter(repo, loader, activator, &fakeAuditor{})
	if perr != nil {
		t.Fatal(perr)
	}
	_, err := p.Promote(context.Background(), "c1", "dev", "rel-old", "")
	if err == nil {
		t.Fatalf("expected error for empty prompt")
	}
}

func TestPromoter_Promote_NoOldReleaseID(t *testing.T) {
	repo, loader, activator := newPromoterSetup(t)
	p, perr := NewPromoter(repo, loader, activator, &fakeAuditor{})
	if perr != nil {
		t.Fatal(perr)
	}
	// No old release id: the promoter probes versions.
	res, err := p.Promote(context.Background(), "c1", "dev", "", "new prompt text")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if res.NewReleaseID == "" {
		t.Errorf("missing new release id")
	}
}

func TestPromoter_LoadActiveManifest_Fallback(t *testing.T) {
	// seedCapability already created a release; loadActiveManifest
	// should pick it up.
	repo, loader, activator := newPromoterSetup(t)
	p, perr := NewPromoter(repo, loader, activator, &fakeAuditor{})
	if perr != nil {
		t.Fatal(perr)
	}
	manifest, err := p.loadActiveManifest(context.Background(), "c1", "rel-old")
	if err != nil {
		t.Fatalf("loadActiveManifest: %v", err)
	}
	if manifest.Prompt.Hash == "" {
		t.Errorf("manifest has no prompt hash")
	}
}

func TestRevision_EmptyCurrentPrompt(t *testing.T) {
	s := NewLLMRevisionStrategy(func(_ context.Context, _ LLMInvokeRequest) (string, error) {
		return "x", nil
	})
	if _, err := s.Revise(context.Background(), ReviseRequest{}); err == nil {
		t.Fatalf("expected error for empty current prompt")
	}
}

func TestRevision_WireFormat(t *testing.T) {
	var gotReq LLMInvokeRequest
	invoke := func(_ context.Context, req LLMInvokeRequest) (string, error) {
		gotReq = req
		return "revised", nil
	}
	s := NewLLMRevisionStrategy(invoke)
	resp, err := s.Revise(context.Background(), ReviseRequest{
		CurrentPrompt:   "old",
		CurrentHash:     "h_old",
		ModelPolicyHash: "h_mp",
		FailingCases: []FailingCase{
			{Seq: 0, Inputs: map[string]any{"q": "ping"}, Expected: "pong", Actual: "wrong"},
		},
	})
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if resp.NewPrompt != "revised" {
		t.Errorf("NewPrompt = %q, want revised", resp.NewPrompt)
	}
	if gotReq.System != DefaultRevisionLLMSystem {
		t.Errorf("System = %q, want DefaultRevisionLLMSystem", gotReq.System)
	}
	// User payload should be valid JSON containing the failing case.
	var payload struct {
		CurrentPrompt string `json:"current_prompt"`
		CurrentHash   string `json:"current_hash"`
		FailingCases  []struct {
			Seq      int    `json:"seq"`
			Expected string `json:"expected"`
		} `json:"failing_cases"`
	}
	if err := json.Unmarshal([]byte(gotReq.User), &payload); err != nil {
		t.Fatalf("user payload not JSON: %v", err)
	}
	if payload.CurrentPrompt != "old" || payload.CurrentHash != "h_old" {
		t.Errorf("payload fields wrong: %+v", payload)
	}
	if len(payload.FailingCases) != 1 || payload.FailingCases[0].Expected != "pong" {
		t.Errorf("failing case not round-tripped: %+v", payload)
	}
}

func TestRevision_InvokeError(t *testing.T) {
	s := NewLLMRevisionStrategy(func(_ context.Context, _ LLMInvokeRequest) (string, error) {
		return "", context.DeadlineExceeded
	})
	_, err := s.Revise(context.Background(), ReviseRequest{CurrentPrompt: "old"})
	if err == nil {
		t.Fatalf("expected error on invoke failure")
	}
}

func TestRevision_NilInvoke(t *testing.T) {
	s := NewLLMRevisionStrategy(nil)
	_, err := s.Revise(context.Background(), ReviseRequest{CurrentPrompt: "old"})
	if err == nil {
		t.Fatalf("expected error for nil invoke")
	}
}
