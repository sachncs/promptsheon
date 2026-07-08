package snapshot

import (
	"context"
	"fmt"
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

func TestSnapshotCorruptJSON(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a row with corrupt JSON in token_usage and metadata
	_, err = db.Exec(`INSERT INTO output_snapshots
		(id, prompt_hash, prompt_text, model, response_text, provider, token_usage, latency_ms, hallucination_score, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"corrupt-1", "hash", "text", "model", "resp", "prov",
		"{invalid json}", 100, 0, `{also invalid}`, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Get should fail due to corrupt token_usage JSON
	_, err = store.Get(ctx, "corrupt-1")
	if err == nil {
		t.Error("expected error for corrupt token_usage JSON")
	}

	// List should also fail
	_, err = store.List(ctx, Filter{PromptHash: "hash"})
	if err == nil {
		t.Error("expected error for corrupt JSON in List")
	}
}

func TestListWithPagination(t *testing.T) {
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
	now := time.Now()
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("snap-%d", i+1)
		snap := &Snapshot{
			ID:           id,
			PromptHash:   "hash",
			PromptText:   "p",
			Model:        "gpt-4",
			ResponseText: "r",
			Provider:     "openai",
			TokenUsage:   TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			LatencyMs:    100,
			CreatedAt:    now.Add(-time.Duration(i) * time.Second),
		}
		if e := store.Save(ctx, snap); e != nil {
			t.Fatal(e)
		}
	}

	// Limit
	snaps, err := store.List(ctx, Filter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}

	// Offset with Limit
	snaps, err = store.List(ctx, Filter{Limit: 10, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}

	// Limit + Offset
	snaps, err = store.List(ctx, Filter{Limit: 1, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestGetNotFound(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Get(context.Background(), "does-not-exist")
	if err == nil {
		t.Error("expected error for non-existent snapshot")
	}
}

func TestListQueryError(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	_ = db.Close()

	_, err = store.List(context.Background(), Filter{})
	if err == nil {
		t.Error("expected error with closed DB")
	}
}

func TestListScanError(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO output_snapshots
		(id, prompt_hash, prompt_text, model, response_text, provider, token_usage, latency_ms, hallucination_score, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad-scan", "h", "t", "m", "r", "p", "{}", "not-an-int", 0, "{}", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.List(context.Background(), Filter{})
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestListCorruptMetadata(t *testing.T) {
	db, err := newTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO output_snapshots
		(id, prompt_hash, prompt_text, model, response_text, provider, token_usage, latency_ms, hallucination_score, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"meta-corrupt", "hash", "text", "model", "resp", "prov",
		"{}", 100, 0, "{bad json}", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	_, err = store.List(ctx, Filter{PromptHash: "hash"})
	if err == nil {
		t.Error("expected error for corrupt metadata JSON in List")
	}

	_, err = store.Get(ctx, "meta-corrupt")
	if err == nil {
		t.Error("expected error for corrupt metadata JSON in Get")
	}
}
