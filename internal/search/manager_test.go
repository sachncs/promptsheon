package search

import (
	"testing"
)

func TestManager_New(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if got := m.Size(); got != 0 {
		t.Fatalf("expected size 0, got %d", got)
	}
}

func TestManager_AddAndSearch(t *testing.T) {
	m := NewManager()
	m.Add(Document{ID: "d1", Content: "hello world"})
	m.Add(Document{ID: "d2", Content: "goodbye world"})
	if got := m.Size(); got != 2 {
		t.Fatalf("expected size 2, got %d", got)
	}
	results := m.Search("hello", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Document.ID != "d1" {
		t.Fatalf("expected d1, got %s", results[0].Document.ID)
	}
}

func TestManager_Remove(t *testing.T) {
	m := NewManager()
	m.Add(Document{ID: "d1", Content: "alpha"})
	m.Add(Document{ID: "d2", Content: "beta"})
	m.Remove("d1")
	if got := m.Size(); got != 1 {
		t.Fatalf("expected size 1, got %d", got)
	}
	if got := m.Search("alpha", 10); len(got) != 0 {
		t.Fatalf("expected 0 results for removed doc, got %d", len(got))
	}
	m.Remove("nonexistent")
	if got := m.Size(); got != 1 {
		t.Fatalf("expected size still 1 after removing nonexistent, got %d", got)
	}
}

func TestManager_Rebuild(t *testing.T) {
	m := NewManager()
	m.Add(Document{ID: "old", Content: "old content"})
	m.Rebuild([]Document{
		{ID: "new1", Content: "new stuff"},
		{ID: "new2", Content: "more new stuff"},
	})
	if got := m.Size(); got != 2 {
		t.Fatalf("expected size 2, got %d", got)
	}
	if got := m.Search("old", 10); len(got) != 0 {
		t.Fatalf("expected no old results, got %d", len(got))
	}
	if got := m.Search("new", 10); len(got) != 2 {
		t.Fatalf("expected 2 new results, got %d", len(got))
	}
}

func TestManager_EmptySearch(t *testing.T) {
	m := NewManager()
	if got := m.Search("test", 10); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestManager_DefaultLimit(t *testing.T) {
	m := NewManager()
	for i := 0; i < 15; i++ {
		m.Add(Document{ID: string(rune('a' + i)), Content: "alpha"})
	}
	results := m.Search("alpha", 0)
	if len(results) != 10 {
		t.Fatalf("expected 10 results with limit=0, got %d", len(results))
	}
	results = m.Search("alpha", -1)
	if len(results) != 10 {
		t.Fatalf("expected 10 results with limit=-1, got %d", len(results))
	}
}
