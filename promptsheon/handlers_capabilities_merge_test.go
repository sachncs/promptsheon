package promptsheon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/capability"
)

// TestHandleUpdateSelfEvolveConfig_PartialPreserves exercises the
// PUT /api/v1/capabilities/{id}/self-evolve endpoint with a
// partial body. Fields the client omits must be preserved from
// the persisted config rather than zeroed — a regression test
// for the bug where `promptsheon selfevolve disable <cap>`
// wiped min_score / max_revisions / cooldown / target_env /
// dataset_id because the handler decoded into a zero-valued
// SelfEvolveConfig and passed it straight to the store.
func TestHandleUpdateSelfEvolveConfig_PartialPreserves(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	// Seed a capability with a known self-evolve config.
	if err := s.db.CreateCapability(ctx, &capability.Capability{
		ID:        "cap-merge",
		ProjectID: "p1",
		Name:      "merge",
		SelfEvolve: capability.SelfEvolveConfig{
			Enabled:      true,
			MinScore:     0.75,
			MaxRevisions: 7,
			CooldownSec:  1234,
			TargetEnv:    "staging",
			DatasetID:    "ds-keep",
		},
	}); err != nil {
		t.Fatalf("seed capability: %v", err)
	}

	// PUT just {"enabled": false}. Other fields must survive.
	body, _ := json.Marshal(map[string]any{"enabled": false})
	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/capabilities/cap-merge/self-evolve",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	got, err := s.db.GetCapability(ctx, "cap-merge")
	if err != nil {
		t.Fatalf("GetCapability: %v", err)
	}
	if got.SelfEvolve.Enabled {
		t.Errorf("expected Enabled=false, got true")
	}
	if got.SelfEvolve.MinScore != 0.75 {
		t.Errorf("MinScore clobbered: want 0.75, got %v", got.SelfEvolve.MinScore)
	}
	if got.SelfEvolve.MaxRevisions != 7 {
		t.Errorf("MaxRevisions clobbered: want 7, got %d", got.SelfEvolve.MaxRevisions)
	}
	if got.SelfEvolve.CooldownSec != 1234 {
		t.Errorf("CooldownSec clobbered: want 1234, got %d", got.SelfEvolve.CooldownSec)
	}
	if got.SelfEvolve.TargetEnv != "staging" {
		t.Errorf("TargetEnv clobbered: want staging, got %q", got.SelfEvolve.TargetEnv)
	}
	if got.SelfEvolve.DatasetID != "ds-keep" {
		t.Errorf("DatasetID clobbered: want ds-keep, got %q", got.SelfEvolve.DatasetID)
	}
}
