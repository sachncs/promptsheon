package capability

// KnowledgeSource represents external knowledge that a capability can use.
//
// Knowledge is versioned independently from the capability — a knowledge
// source can be updated (re-indexed, re-embedded, new documents) without
// creating a new capability version.
type KnowledgeSource struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           string         `json:"type"` // "documents", "index", "rag", "graph", "sql", "files"
	Version        string         `json:"version"`
	EmbeddingModel string         `json:"embedding_model,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}
