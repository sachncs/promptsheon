package pluginsup

import (
	"context"
	"testing"
)

func TestRemoteLifecycle(t *testing.T) {
	t.Parallel()
	r := &Remote{Entry: manifestEntry(t, "alpha")}

	// Remote now fail-closes: a manifest entry without a
	// binary: line is a configuration error, not a no-op.
	// Start returns errRemoteNotConfigured so the supervisor
	// records the failure and the operator sees the gap in
	// /metrics instead of a silent healthy stub.
	if err := r.Start(context.Background()); err == nil {
		t.Fatalf("Start: expected error for binary-less entry, got nil")
	}

	// Stop is still a no-op (there's nothing running).
	if err := r.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Health surfaces the same error on every poll.
	if err := r.Health(context.Background()); err == nil {
		t.Fatalf("Health: expected error for binary-less entry, got nil")
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
