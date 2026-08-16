package evolve

import (
	"testing"
)

func TestGenerateID_Format(t *testing.T) {
	id := generateID("v")
	if len(id) < 3 {
		t.Fatalf("id too short: %q", id)
	}
	if id[:2] != "v-" {
		t.Errorf("id = %q, want prefix v-", id)
	}
	// 8 random bytes hex-encoded = 16 chars after the dash.
	if len(id) != 18 {
		t.Errorf("id = %q, want 18 chars", id)
	}
	// Two calls return different ids (crypto/rand).
	first := generateID("x")
	second := generateID("x")
	if first == second {
		t.Errorf("generateID returned the same value twice: %q", first)
	}
}
