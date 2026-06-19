// Package search provides semantic search capabilities using vector embeddings.
package search

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"promptsheon/internal/models"
)

// Embedding represents a vector embedding.
type Embedding []float64

// Document represents an indexed document for search.
type Document struct {
	ID        string    `json:"id"`
	PromptID  string    `json:"prompt_id"`
	Content   string    `json:"content"`
	Embedding Embedding `json:"embedding"`
	Metadata  map[string]string `json:"metadata,omitempty"`
 IndexedAt time.Time `json:"indexed_at"`
}

// SearchResult represents a search result with relevance score.
type SearchResult struct {
	Document *Document `json:"document"`
	Score    float64   `json:"score"` // 0-1, higher is more relevant
}

// Index manages the vector index for semantic search.
type Index struct {
	mu        sync.RWMutex
	documents map[string]*Document
	dimensions int
}

// NewIndex creates a new search index.
func NewIndex(dimensions int) *Index {
	if dimensions <= 0 {
		dimensions = 128 // Default dimensions
	}
	return &Index{
		documents:  make(map[string]*Document),
		dimensions: dimensions,
	}
}

// GenerateEmbedding generates a simple embedding for demonstration.
// In production, this would call an embedding model API.
func GenerateEmbedding(text string, dimensions int) Embedding {
	embedding := make(Embedding, dimensions)
	
	// Simple hash-based embedding (for demo purposes)
	// In production, use a real embedding model
	words := strings.Fields(strings.ToLower(text))
	for i, word := range words {
		idx := i % dimensions
		// Simple hash of word
		hash := 0
		for _, c := range word {
			hash = hash*31 + int(c)
		}
		embedding[idx] += float64(hash%1000) / 1000.0
	}
	
	// Normalize
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	
	return embedding
}

// CosineSimilarity calculates the cosine similarity between two embeddings.
func CosineSimilarity(a, b Embedding) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// IndexDocument adds or updates a document in the index.
func (idx *Index) IndexDocument(doc *Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	if doc.Embedding == nil {
		doc.Embedding = GenerateEmbedding(doc.Content, idx.dimensions)
	}
	doc.IndexedAt = time.Now()
	idx.documents[doc.ID] = doc
}

// RemoveDocument removes a document from the index.
func (idx *Index) RemoveDocument(id string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	if _, exists := idx.documents[id]; exists {
		delete(idx.documents, id)
		return true
	}
	return false
}

// Search performs a semantic search against the index.
func (idx *Index) Search(query Embedding, limit int) []*SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	if limit <= 0 {
		limit = 10
	}
	
	results := make([]*SearchResult, 0, len(idx.documents))
	
	for _, doc := range idx.documents {
		score := CosineSimilarity(query, doc.Embedding)
		results = append(results, &SearchResult{
			Document: doc,
			Score:    score,
		})
	}
	
	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	if len(results) > limit {
		results = results[:limit]
	}
	
	return results
}

// SearchByText performs semantic search using text query.
func (idx *Index) SearchByText(query string, limit int) []*SearchResult {
	queryEmbedding := GenerateEmbedding(query, idx.dimensions)
	return idx.Search(queryEmbedding, limit)
}

// GetDocument retrieves a document by ID.
func (idx *Index) GetDocument(id string) (*Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	doc, exists := idx.documents[id]
	return doc, exists
}

// Size returns the number of indexed documents.
func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.documents)
}

// Clear removes all documents from the index.
func (idx *Index) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.documents = make(map[string]*Document)
}

// Indexer manages prompt indexing and search.
type Indexer struct {
	index *Index
}

// NewIndexer creates a new indexer.
func NewIndexer() *Indexer {
	return &Indexer{
		index: NewIndex(128),
	}
}

// IndexPrompt indexes a prompt for semantic search.
func (i *Indexer) IndexPrompt(prompt *models.Prompt) {
	content := prompt.Content
	if prompt.Name != "" {
		content = prompt.Name + " " + content
	}
	
	doc := &Document{
		ID:       prompt.ID,
		PromptID: prompt.ID,
		Content:  content,
		Metadata: map[string]string{
			"name":   prompt.Name,
			"status": string(prompt.Status),
		},
	}
	
	i.index.IndexDocument(doc)
}

// SearchPrompts searches for prompts using semantic similarity.
func (i *Indexer) SearchPrompts(query string, limit int) []*SearchResult {
	return i.index.SearchByText(query, limit)
}

// GetIndexStats returns statistics about the index.
func (i *Indexer) GetIndexStats() map[string]any {
	return map[string]any{
		"total_documents": i.index.Size(),
		"dimensions":      i.index.dimensions,
	}
}
