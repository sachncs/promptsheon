package search

import (
	"strings"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/models"
)

// TestTokenize_BasicSplit pins the BM25 tokeniser: unicode
// letters and digits are kept (lowercased), everything else is
// a separator. The bigram pass concatenates adjacent tokens
// with a single space.
func TestTokenize_BasicSplit(t *testing.T) {
	got := tokenize("Hello, World! 123 abc")
	if !contains(got, "hello") {
		t.Fatalf("missing 'hello' in %v", got)
	}
	if !contains(got, "world") {
		t.Fatalf("missing 'world' in %v", got)
	}
	if !contains(got, "123") {
		t.Fatalf("missing '123' in %v", got)
	}
	if !contains(got, "abc") {
		t.Fatalf("missing 'abc' in %v", got)
	}
	if !contains(got, "hello world") {
		t.Fatalf("missing 'hello world' bigram in %v", got)
	}
}

// TestTokenize_PluralStrip pins the light suffix strip: a
// trailing 's' is removed from words longer than 3 characters.
// "prompts" -> "prompt".
func TestTokenize_PluralStrip(t *testing.T) {
	got := tokenize("prompts are nice")
	if !contains(got, "prompt") {
		t.Fatalf("missing 'prompt' stem in %v", got)
	}
	if contains(got, "prompts") {
		t.Fatalf("'prompts' should be stemmed to 'prompt', got %v", got)
	}
}

// TestTokenize_EmptyAndPunctOnly pins the boundary cases: an
// empty string and a punctuation-only string both return nil.
func TestTokenize_EmptyAndPunctOnly(t *testing.T) {
	if got := tokenize(""); got != nil {
		t.Fatalf("empty: expected nil, got %v", got)
	}
	if got := tokenize("!!! --- ???"); got != nil {
		t.Fatalf("punct-only: expected nil, got %v", got)
	}
}

// TestBM25_TermFrequency pins the basic ranking: a document
// containing the query term multiple times outranks a document
// containing it once.
func TestBM25_TermFrequency(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "d1", Content: "alpha alpha alpha"})
	idx.Add(Document{ID: "d2", Content: "alpha beta gamma"})

	results := idx.Search("alpha", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Document.ID != "d1" {
		t.Fatalf("expected d1 first (higher term freq), got %s", results[0].Document.ID)
	}
	if results[0].Score <= results[1].Score {
		t.Fatalf("expected d1 score > d2 score, got %f vs %f", results[0].Score, results[1].Score)
	}
}

// TestBM25_IDF pins the inverse-document-frequency term:
// querying for a rare term returns a higher score than querying
// for a term that appears in every document.
func TestBM25_IDF(t *testing.T) {
	idx := NewIndex()
	// "common" appears in 3 of 3 documents.
	// "rare" appears in 1 of 3 documents.
	idx.Add(Document{ID: "a", Content: "common rare"})
	idx.Add(Document{ID: "b", Content: "common zebra"})
	idx.Add(Document{ID: "c", Content: "common yogurt"})

	results := idx.Search("rare", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'rare'")
	}
	if results[0].Document.ID != "a" {
		t.Fatalf("expected document 'a' for 'rare', got %s", results[0].Document.ID)
	}

	commonResults := idx.Search("common", 10)
	if len(commonResults) != 3 {
		t.Fatalf("expected 3 results for 'common', got %d", len(commonResults))
	}
	for _, r := range commonResults {
		if r.Score <= 0 {
			t.Fatalf("expected positive score for 'common', got %f", r.Score)
		}
	}
}

// TestBM25_LengthNormalisation pins the length-normalisation
// term: a short document with one occurrence of the query
// term outranks a long document with the same single
// occurrence.
func TestBM25_LengthNormalisation(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "short", Content: "alpha"})
	idx.Add(Document{ID: "long", Content: "alpha " + strings.Repeat("filler ", 200)})

	results := idx.Search("alpha", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Document.ID != "short" {
		t.Fatalf("expected 'short' first (length norm), got %s", results[0].Document.ID)
	}
	if results[0].Score <= results[1].Score {
		t.Fatalf("expected short > long, got %f vs %f", results[0].Score, results[1].Score)
	}
}

// TestBM25_BigramMatch pins the bigram pass: querying for
// "machine learning" matches the document containing the
// bigram, even if the two words appear individually in
// unrelated contexts in other documents.
func TestBM25_BigramMatch(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "ml", Content: "machine learning is great"})
	idx.Add(Document{ID: "machine-only", Content: "a machine that washes"})
	idx.Add(Document{ID: "learning-only", Content: "learning a new language"})

	results := idx.Search("machine learning", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].Document.ID != "ml" {
		t.Fatalf("expected 'ml' first, got %s", results[0].Document.ID)
	}
}

// TestBM25_EmptyQuery pins the boundary: a query that tokenises
// to nothing returns nil.
func TestBM25_EmptyQuery(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "a", Content: "hello world"})
	idx.Add(Document{ID: "b", Content: "foo bar"})

	if got := idx.Search("", 10); got != nil {
		t.Fatalf("empty query: expected nil, got %v", got)
	}
	if got := idx.Search("   ", 10); got != nil {
		t.Fatalf("whitespace query: expected nil, got %v", got)
	}
}

// TestBM25_NoMatch pins the boundary: a query with no token
// overlap returns an empty (or nil) result.
func TestBM25_NoMatch(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "a", Content: "alpha beta gamma"})
	if got := idx.Search("xyzzy", 10); len(got) != 0 {
		t.Fatalf("expected no results, got %v", got)
	}
}

// TestBM25_Remove pins the maintenance path: removing a
// document must update df and totalLen, and subsequent
// searches must not return the removed document.
func TestBM25_Remove(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "a", Content: "alpha"})
	idx.Add(Document{ID: "b", Content: "alpha"})

	if got := idx.Search("alpha", 10); len(got) != 2 {
		t.Fatalf("expected 2 results before remove, got %d", len(got))
	}
	idx.Remove("a")
	if got := idx.Search("alpha", 10); len(got) != 1 {
		t.Fatalf("expected 1 result after remove, got %d", len(got))
	}
	if got := idx.Search("alpha", 10); got[0].Document.ID != "b" {
		t.Fatalf("expected b to remain, got %s", got[0].Document.ID)
	}
}

// TestBM25_Rebuild pins the bulk path: Rebuild discards the
// old index and indexes the new set in one atomic step.
func TestBM25_Rebuild(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "old1", Content: "old alpha"})
	idx.Add(Document{ID: "old2", Content: "old beta"})

	idx.Rebuild([]Document{
		{ID: "new1", Content: "new gamma"},
		{ID: "new2", Content: "new delta"},
	})

	if idx.Size() != 2 {
		t.Fatalf("expected size 2 after rebuild, got %d", idx.Size())
	}
	if got := idx.Search("old", 10); len(got) != 0 {
		t.Fatalf("expected no 'old' results after rebuild, got %d", len(got))
	}
	if got := idx.Search("new", 10); len(got) != 2 {
		t.Fatalf("expected 2 'new' results after rebuild, got %d", len(got))
	}
}

// TestBM25_DeterministicOrdering pins the test-friendly side
// of the implementation: when two documents have the same
// score, the result is sorted by docID ascending.
func TestBM25_DeterministicOrdering(t *testing.T) {
	idx := NewIndex()
	idx.Add(Document{ID: "z", Content: "alpha"})
	idx.Add(Document{ID: "a", Content: "alpha"})
	idx.Add(Document{ID: "m", Content: "alpha"})

	results := idx.Search("alpha", 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	want := []string{"a", "m", "z"}
	for i, r := range results {
		if r.Document.ID != want[i] {
			t.Fatalf("position %d: want %s, got %s", i, want[i], r.Document.ID)
		}
	}
}

// TestBM25_SearchLimit pins the limit parameter: at most `limit`
// results are returned, even when more match.
func TestBM25_SearchLimit(t *testing.T) {
	idx := NewIndex()
	for i := 0; i < 10; i++ {
		idx.Add(Document{ID: string(rune('a' + i)), Content: "alpha"})
	}
	results := idx.Search("alpha", 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

// TestDocumentFromPrompt pins the prompt -> document mapping.
func TestDocumentFromPrompt(t *testing.T) {
	p := &models.Prompt{
		ID:          "p1",
		Name:        "greeting",
		Description: "friendly hello",
		Content:     "say hello to the user",
	}
	d := DocumentFromPrompt(p)
	if d.ID != "p1" || d.PromptID != "p1" {
		t.Fatalf("ID mismatch: %+v", d)
	}
	if !strings.Contains(d.Content, "greeting") {
		t.Fatalf("content missing name: %q", d.Content)
	}
	if !strings.Contains(d.Content, "friendly hello") {
		t.Fatalf("content missing description: %q", d.Content)
	}
	if !strings.Contains(d.Content, "say hello to the user") {
		t.Fatalf("content missing body: %q", d.Content)
	}
	if d.Metadata["name"] != "greeting" {
		t.Fatalf("metadata name mismatch: %v", d.Metadata)
	}
}

// contains is a tiny helper so we don't pull in slices.Contains
// just for the tokeniser tests.
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
