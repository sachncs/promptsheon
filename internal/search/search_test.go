package search_test

import (
	"testing"

	"promptsheon/internal/search"
	"promptsheon/internal/models"
)

func TestGenerateEmbedding(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		dims   int
	}{
		{"short text", "hello world", 128},
		{"long text", "This is a longer text for testing embeddings", 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emb := search.GenerateEmbedding(tt.text, tt.dims)
			if len(emb) != tt.dims {
				t.Errorf("expected embedding of size %d, got %d", tt.dims, len(emb))
			}
			// Check normalization
			var norm float64
			for _, v := range emb {
				norm += v * v
			}
			if norm < 0.99 || norm > 1.01 {
				t.Errorf("expected normalized embedding, got norm %f", norm)
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := search.Embedding{1, 0, 0}
	b := search.Embedding{1, 0, 0}
	c := search.Embedding{0, 1, 0}

	sim := search.CosineSimilarity(a, b)
	if sim < 0.99 {
		t.Errorf("expected similarity near 1 for identical vectors, got %f", sim)
	}

	sim = search.CosineSimilarity(a, c)
	if sim > 0.01 {
		t.Errorf("expected similarity near 0 for orthogonal vectors, got %f", sim)
	}
}

func TestIndexDocument(t *testing.T) {
	idx := search.NewIndex(128)

	doc := &search.Document{
		ID:       "doc1",
		PromptID: "prompt1",
		Content:  "Hello world",
	}

	idx.IndexDocument(doc)

	if idx.Size() != 1 {
		t.Errorf("expected size 1, got %d", idx.Size())
	}

	got, exists := idx.GetDocument("doc1")
	if !exists {
		t.Fatal("expected to find document")
	}
	if got.ID != "doc1" {
		t.Errorf("expected ID doc1, got %s", got.ID)
	}
}

func TestRemoveDocument(t *testing.T) {
	idx := search.NewIndex(128)

	idx.IndexDocument(&search.Document{ID: "doc1", Content: "test"})
	idx.IndexDocument(&search.Document{ID: "doc2", Content: "test"})

	if idx.Size() != 2 {
		t.Fatalf("expected size 2, got %d", idx.Size())
	}

	removed := idx.RemoveDocument("doc1")
	if !removed {
		t.Error("expected document to be removed")
	}
	if idx.Size() != 1 {
		t.Errorf("expected size 1, got %d", idx.Size())
	}
}

func TestSearch(t *testing.T) {
	idx := search.NewIndex(128)

	idx.IndexDocument(&search.Document{ID: "doc1", Content: "hello world testing"})
	idx.IndexDocument(&search.Document{ID: "doc2", Content: "goodbye world testing"})
	idx.IndexDocument(&search.Document{ID: "doc3", Content: "completely different content"})

	results := idx.SearchByText("hello", 10)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// First result should have highest score
	if results[0].Score < results[len(results)-1].Score {
		t.Error("expected results sorted by score descending")
	}
}

func TestSearchLimit(t *testing.T) {
	idx := search.NewIndex(128)

	for i := 0; i < 20; i++ {
		idx.IndexDocument(&search.Document{
			ID:      "doc" + string(rune('0'+i)),
			Content: "test document",
		})
	}

	results := idx.SearchByText("test", 5)
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

func TestClear(t *testing.T) {
	idx := search.NewIndex(128)

	idx.IndexDocument(&search.Document{ID: "doc1", Content: "test"})
	idx.IndexDocument(&search.Document{ID: "doc2", Content: "test"})

	idx.Clear()

	if idx.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", idx.Size())
	}
}

func TestIndexer(t *testing.T) {
	indexer := search.NewIndexer()

	prompt := &models.Prompt{
		ID:      "prompt1",
		Name:    "Test Prompt",
		Content: "This is a test prompt",
	}

	indexer.IndexPrompt(prompt)

	stats := indexer.GetIndexStats()
	if stats["total_documents"] != 1 {
		t.Errorf("expected 1 document, got %v", stats["total_documents"])
	}

	results := indexer.SearchPrompts("test", 10)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}
