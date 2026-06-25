package search

import (
	"sync"

	"promptsheon/internal/models"
)

// Manager owns a long-lived in-memory search index. M-1 fix: the
// previous handlers rebuilt the index on every search request,
// which is O(N) per query and unbounded in memory. The Manager
// keeps the index in memory and is refreshed on prompt mutations
// (Add, Remove). The underlying embeddings are still a hash stub;
// future work can swap in a real embedding model behind the same
// interface.
//
// Concurrency: reads (Search, Size) take a read lock; writes
// (Add, Remove, Clear, Rebuild) take a write lock. The Manager is
// safe for use from multiple goroutines.
type Manager struct {
	mu   sync.RWMutex
	docs map[string]*Document
	dim  int
}

// NewManager creates a new Manager with the given embedding
// dimension. Pass 0 or negative for the default (128).
func NewManager(dim int) *Manager {
	if dim <= 0 {
		dim = 128
	}
	return &Manager{
		docs: make(map[string]*Document),
		dim:  dim,
	}
}

// Add indexes a single prompt. If a document with the same ID
// already exists it is replaced.
func (m *Manager) Add(p *models.Prompt) {
	if p == nil {
		return
	}
	doc := &Document{
		ID:       p.ID,
		PromptID: p.ID,
		Content:  p.Content,
		Metadata: map[string]string{
			"name":   p.Name,
			"status": string(p.Status),
		},
	}
	doc.Embedding = GenerateEmbedding(doc.Content, m.dim)
	m.mu.Lock()
	m.docs[doc.ID] = doc
	m.mu.Unlock()
}

// Remove deletes the document with the given ID from the index.
// No-op if the document does not exist.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	delete(m.docs, id)
	m.mu.Unlock()
}

// Rebuild replaces the entire index with the given prompts. The
// old index is discarded atomically; readers see either the old
// or new set but not a mix.
func (m *Manager) Rebuild(prompts []*models.Prompt) {
	next := make(map[string]*Document, len(prompts))
	for _, p := range prompts {
		doc := &Document{
			ID:       p.ID,
			PromptID: p.ID,
			Content:  p.Content,
			Metadata: map[string]string{
				"name":   p.Name,
				"status": string(p.Status),
			},
		}
		doc.Embedding = GenerateEmbedding(doc.Content, m.dim)
		next[doc.ID] = doc
	}
	m.mu.Lock()
	m.docs = next
	m.mu.Unlock()
}

// Search runs a semantic search and returns the top limit results.
func (m *Manager) Search(query string, limit int) []*SearchResult {
	if limit <= 0 {
		limit = 10
	}
	q := GenerateEmbedding(query, m.dim)
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]*SearchResult, 0, len(m.docs))
	for _, doc := range m.docs {
		results = append(results, &SearchResult{
			Document: doc,
			Score:    CosineSimilarity(q, doc.Embedding),
		})
	}
	// Sort by score descending. For small N this is fine; the
	// previous handler did the same thing on every request.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Size returns the number of indexed documents.
func (m *Manager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.docs)
}

// Dimension returns the configured embedding dimension.
func (m *Manager) Dimension() int { return m.dim }
