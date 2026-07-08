// Package search provides text search and indexing for prompts and capabilities.
package search

import (
	"sync"
)

// Manager manages the search index with concurrent access control.
type Manager struct {
	mu  sync.RWMutex
	idx *Index
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{idx: NewIndex()}
}

// Add adds a document to the search index.
func (m *Manager) Add(doc Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Add(doc)
}

// Remove removes a document from the search index by ID.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Remove(id)
}

// Rebuild replaces the search index with the given documents.
func (m *Manager) Rebuild(docs []Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Rebuild(docs)
}

// Search queries the index for documents matching the query string.
func (m *Manager) Search(query string, limit int) []Result {
	if limit <= 0 {
		limit = 10
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Search(query, limit)
}

// Size returns the number of documents in the index.
func (m *Manager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Size()
}
