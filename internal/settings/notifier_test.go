// Notifier propagation tests. The CRDT-aware settings layer
// raises an error from Publish so a subscriber that fails to
// consume the new value (e.g. the vault refusing a hot-reload
// key) surfaces to the API caller as 5xx. This file pins the
// propagation contract without storing vault master keys in
// settings — the test wires the vault directly into the
// notifier, bypassing the settings layer's storage path.
package settings

import (
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/internal/vault"
)

func TestNotifier_PublishReturnsSubscriberError(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	want := errors.New("vault refused")
	n.Subscribe("vault.key", func(_ string) error { return want })

	if err := n.Publish("vault.key", "anything"); !errors.Is(err, want) {
		t.Fatalf("Publish: got %v want %v", err, want)
	}
}

func TestNotifier_PublishStopsOnFirstError(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	var secondCalled bool
	n.Subscribe("vault.key", func(_ string) error { return errors.New("first failed") })
	n.Subscribe("vault.key", func(_ string) error { secondCalled = true; return nil })
	if err := n.Publish("vault.key", "x"); err == nil {
		t.Fatal("expected first subscriber to fail")
	}
	if secondCalled {
		t.Fatal("second subscriber should not be called after first fails")
	}
}

func TestNotifier_VaultReloadFailurePropagates(t *testing.T) {
	t.Parallel()
	// Vault.Reload already returns an error; the notifier
	// must surface that error so the API handler can return
	// 5xx. This test does NOT store the vault master key in
	// settings; it wires the Vault directly into the
	// notifier so the propagation path is the only thing
	// under test.
	n := NewNotifier()
	v, err := vault.New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	n.Subscribe("vault.key", func(value string) error {
		return v.Reload(value)
	})
	// Reload with an invalid key — Vault.Reload returns an
	// error and the notifier surfaces it.
	err = n.Publish("vault.key", "not-valid-hex")
	if err == nil {
		t.Fatal("expected vault Reload to fail and propagate through the notifier")
	}
	// Reload with a valid key — no error, the original key
	// is retained and the notifier returns nil.
	if err := n.Publish("vault.key", "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"); err != nil {
		t.Fatalf("valid reload: %v", err)
	}
}

func TestNotifier_NoSubscribers(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	if err := n.Publish("vault.key", "x"); err != nil {
		t.Fatalf("no subscribers should not error, got %v", err)
	}
}

func TestNotifier_MultipleSubscribersAllRun(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	calls := 0
	n.Subscribe("k", func(_ string) error { calls++; return nil })
	n.Subscribe("k", func(_ string) error { calls++; return nil })
	if err := n.Publish("k", "v"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
