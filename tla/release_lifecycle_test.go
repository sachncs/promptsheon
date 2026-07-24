package tla

import (
	"os"
	"strings"
	"testing"
)

// TestReleaseLifecycleSpecExists pins TLA-LIFECYCLE-1: a TLA+
// spec for the release lifecycle state machine ships at
// tla/release_lifecycle.tla with a matching TLC config at
// tla/release_lifecycle.cfg. The spec models the
// Maker/Checker separation-of-duties invariant and the
// "exactly one active release per Environment" invariant
// the runtime Activate path enforces.
func TestReleaseLifecycleSpecExists(t *testing.T) {
	spec, err := os.ReadFile("release_lifecycle.tla")
	if err != nil {
		t.Fatalf("read release_lifecycle.tla: %v", err)
	}
	cfg, err := os.ReadFile("release_lifecycle.cfg")
	if err != nil {
		t.Fatalf("read release_lifecycle.cfg: %v", err)
	}
	specText := string(spec)
	cfgText := string(cfg)

	required := []string{
		"MODULE release_lifecycle",
		"ActiveExclusive",
		"StatusConsistency",
		"NoActiveToSupersede",
		"VoteImpliesPendingOrApproved",
		"Approve(r)",
		"Activate(r, env)",
		"Supersede(r, env)",
		"Rollback(r)",
		"MakerChecker",
	}
	for _, fragment := range required {
		if !strings.Contains(specText, fragment) {
			t.Errorf("release_lifecycle.tla must contain %q", fragment)
		}
	}
	cfgRequired := []string{
		"SPECIFICATION Spec",
		"Releases = ",
		"Environments = ",
		"MaxReleases = ",
	}
	for _, fragment := range cfgRequired {
		if !strings.Contains(cfgText, fragment) {
			t.Errorf("release_lifecycle.cfg must contain %q", fragment)
		}
	}
}
