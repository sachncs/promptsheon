package search

import (
	"sync"
)

type Manager struct {
	mu  sync.RWMutex
	idx *Index
}

func NewManager() *Manager {
	return &Manager{idx: NewIndex()}
}

func (m *Manager) Add(doc Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Add(doc)
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Remove(id)
}

func (m *Manager) Rebuild(docs []Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Rebuild(docs)
}

func (m *Manager) Search(query string, limit int) []Result {
	if limit <= 0 {
		limit = 10
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Search(query, limit)
}

func (m *Manager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idx.Size()
}
