package pluginsup

import (
	"context"
	"testing"
)

func TestRemoteLifecycle(t *testing.T) {
	t.Parallel()
	r := &Remote{Entry: manifestEntry(t, "alpha")}
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := r.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestRemoteUDSOverride(t *testing.T) {
	// v0.1.0: the UDS path is computed in LoadFromEnv, not on
	// the Remote type. The fact that the manifest entry's UDS
	// is preserved is what the test asserts.
	e := manifestEntryWithUDS(t, "beta", "/custom/path.sock")
	if e.UDS != "/custom/path.sock" {
		t.Fatalf("expected /custom/path.sock, got %s", e.UDS)
	}
}
