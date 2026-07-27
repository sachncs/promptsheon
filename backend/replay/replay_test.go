package replay

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHashDeterministic(t *testing.T) {
	t.Parallel()
	in := HashInput{
		WorkspaceID:   "ws-1",
		ReleaseID:     "rel-1",
		ManifestHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		InputHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Model:         "gpt-4o",
		ModelRevision: "2024-08",
	}
	a, err := Hash(in)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	b, err := Hash(in)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if a != b {
		t.Fatalf("hash not deterministic: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("expected 64-char hex, got %d", len(a))
	}
}

func TestHashChangesWithAnyField(t *testing.T) {
	t.Parallel()
	base := HashInput{
		WorkspaceID:   "ws-1",
		ReleaseID:     "rel-1",
		ManifestHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		InputHash:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Model:         "gpt-4o",
		ModelRevision: "2024-08",
	}
	baseHash, _ := Hash(base)
	for _, mut := range []HashInput{
		{WorkspaceID: "ws-2"},
		{ReleaseID: "rel-2"},
		{ManifestHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{InputHash: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
		{Model: "gpt-4o-mini"},
		{ModelRevision: "2024-09"},
	} {
		merged := base
		switch {
		case mut.WorkspaceID != "":
			merged.WorkspaceID = mut.WorkspaceID
		case mut.ReleaseID != "":
			merged.ReleaseID = mut.ReleaseID
		case mut.ManifestHash != "":
			merged.ManifestHash = mut.ManifestHash
		case mut.InputHash != "":
			merged.InputHash = mut.InputHash
		case mut.Model != "":
			merged.Model = mut.Model
		case mut.ModelRevision != "":
			merged.ModelRevision = mut.ModelRevision
		}
		got, _ := Hash(merged)
		if got == baseHash {
			t.Fatalf("hash should differ when field is mutated; still %s", got)
		}
	}
}

func TestRecordForSetsHashAndTimestamp(t *testing.T) {
	t.Parallel()
	in := json.RawMessage(`{"q":"hi"}`)
	out := json.RawMessage(`{"a":"hello"}`)
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	r, err := RecordFor("ws", "rel", "hhhh", "iiii", "gpt-4o", "2024-08", in, out, 10, 20, 0.0001, 100, "ok", "", at)
	if err != nil {
		t.Fatalf("RecordFor: %v", err)
	}
	if r.ExecutionHash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if !r.CapturedAt.Equal(at) {
		t.Fatalf("expected CapturedAt=%v got %v", at, r.CapturedAt)
	}
	if r.Status != "ok" {
		t.Fatalf("expected status=ok, got %s", r.Status)
	}
}

func TestRecordForHashIdempotent(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	in := json.RawMessage(`{}`)
	out := json.RawMessage(`{}`)
	a, _ := RecordFor("ws", "rel", "hhhh", "iiii", "gpt-4o", "2024-08", in, out, 0, 0, 0, 0, "ok", "", at)
	b, _ := RecordFor("ws", "rel", "hhhh", "iiii", "gpt-4o", "2024-08", in, out, 0, 0, 0, 0, "ok", "", at)
	if a.ExecutionHash != b.ExecutionHash {
		t.Fatalf("expected equal hashes, got %s vs %s", a.ExecutionHash, b.ExecutionHash)
	}
}
