package snapshot

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotStore(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	snap := &Snapshot{
		ID:           "test-snap-1",
		PromptHash:   "hash-abc",
		PromptText:   "You are a helpful assistant. Answer: {{question}}",
		Model:        "gpt-4",
		ResponseText: "The answer is 42.",
		Provider:     "openai",
		TokenUsage:   TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		LatencyMs:    150,
		Metadata:     map[string]string{"user_id": "u1"},
		CreatedAt:    time.Now(),
	}

	// Save
	if e := store.Save(ctx, snap); e != nil {
		t.Fatal(e)
	}

	// Get
	got, err := store.Get(ctx, "test-snap-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ResponseText != "The answer is 42." {
		t.Fatalf("expected response text, got %q", got.ResponseText)
	}
	if got.TokenUsage.TotalTokens != 15 {
		t.Fatalf("expected 15 tokens, got %d", got.TokenUsage.TotalTokens)
	}

	// List by prompt hash
	snaps, err := store.List(ctx, Filter{PromptHash: "hash-abc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	// List by model
	snaps, err = store.List(ctx, Filter{Model: "gpt-4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot by model, got %d", len(snaps))
	}

	// List with no results
	snaps, err = store.List(ctx, Filter{Model: "nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(snaps))
	}
}
