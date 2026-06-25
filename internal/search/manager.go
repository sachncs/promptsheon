package search

import (
	"sync"

	"promptsheon/internal/models"
)

// Manager owns a long-lived BM25 search index. M-1 fix kept the
// per-Server in-memory state; the embedding stub has now been
// replaced with a real BM25 ranking function (see bm25.go).
// Concurrency: reads (Search, Size) take a read lock on the
// underlying Index; writes (Add, Remove, Rebuild) take a write
// lock. The Manager is safe for use from multiple goroutines.
type Manager struct {
	mu sync.RWMutex
	idx *Index
}

// NewManager creates a new Manager backed by a fresh BM25 Index.
func NewManager() *Manager {
	return &Manager{idx: NewIndex()}
}

// Add indexes a single prompt. If a document with the same ID
// already exists it is replaced.
func (m *Manager) Add(p *models.Prompt) {
	if p == nil {
		return
	}
	doc := DocumentFromPrompt(p)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Add(doc)
}

// Remove deletes the document with the given ID from the index.
// No-op if the document does not exist.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Remove(id)
}

// Rebuild replaces the entire index with the given prompts. The
// old index is discarded atomically; readers see either the old
// or new set but not a mix.
func (m *Manager) Rebuild(prompts []*models.Prompt) {
	docs := make([]Document, len(prompts))
	for i, p := range prompts {
		docs[i] = DocumentFromPrompt(p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Rebuild(docs)
}

// Search runs a BM25 search and returns the top limit results
// ranked by score (descending).
func (m *Manager) Search(query string, limit int) []Result {
	if limit <= 0 {
		limit = 10
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Search(query, limit)
}

// Size returns the number of indexed documents.
func (m *Manager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Size()
}
