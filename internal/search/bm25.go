// Package search provides a BM25-based full-text search index for
// prompts. The ranking function is the standard BM25 (k1=1.2,
// b=0.75) with token unigrams and bigrams, a light suffix
// strip for crude plural handling, and per-document length
// normalisation. The Manager type wraps an Index with a
// thread-safe API used by internal/api.
package search

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// bm25K1 and bm25B are the standard BM25 hyperparameters. These
// values are the de-facto defaults from the original BM25 papers
// and produce well-behaved rankings on English text without any
// tuning.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// Result is a single BM25 ranking result. Score is the BM25 score;
// higher means more relevant. Document is the indexed entry.
type Result struct {
	Document *Document
	Score    float64
}

// Document is a single indexed entry. The prompt's content is
// indexed; metadata is preserved for display in search results.
type Document struct {
	ID        string            `json:"id"`
	PromptID  string            `json:"prompt_id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	IndexedAt int64             `json:"indexed_at"`
}

// Index is a thread-safe BM25 index. Add/Remove/Rebuild take the
// write lock; Search takes the read lock.
type Index struct {
	mu   sync.RWMutex
	docs map[string]*indexedDoc
	// docLenCache maps docID -> length-in-tokens.
	docLenCache map[string]int
	// df maps term -> number of documents containing it.
	df map[string]int
	// totalLen is the sum of all doc lengths.
	totalLen int
}

type indexedDoc struct {
	doc    Document
	tokens []string
	// tf maps term -> term frequency in this doc.
	tf map[string]int
}

// NewIndex creates an empty BM25 index.
func NewIndex() *Index {
	return &Index{
		docs:        make(map[string]*indexedDoc),
		docLenCache: make(map[string]int),
		df:          make(map[string]int),
	}
}

// Add indexes a single document. If a document with the same ID
// already exists, it is replaced (the old document's contribution
// to df and totalLen is removed first).
func (idx *Index) Add(doc Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.addLocked(doc)
}

// addLocked is Add without the lock acquisition; callers that
// already hold the write lock use this.
func (idx *Index) addLocked(doc Document) {
	if old, ok := idx.docs[doc.ID]; ok {
		idx.removeLocked(old)
	}
	tokens := tokenize(doc.Content)
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	idx.docs[doc.ID] = &indexedDoc{
		doc:    doc,
		tokens: tokens,
		tf:     tf,
	}
	idx.docLenCache[doc.ID] = len(tokens)
	idx.totalLen += len(tokens)
	for term := range tf {
		idx.df[term]++
	}
}

// Remove deletes the document with the given ID. No-op if it
// does not exist.
func (idx *Index) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if old, ok := idx.docs[id]; ok {
		idx.removeLocked(old)
	}
}

// removeLocked is Remove without the lock acquisition.
func (idx *Index) removeLocked(d *indexedDoc) {
	delete(idx.docs, d.doc.ID)
	idx.totalLen -= len(d.tokens)
	delete(idx.docLenCache, d.doc.ID)
	for term := range d.tf {
		idx.df[term]--
		if idx.df[term] == 0 {
			delete(idx.df, term)
		}
	}
}

// Rebuild replaces the entire index with the given documents. The
// old index is discarded atomically; readers see either the old
// or new set but not a mix.
func (idx *Index) Rebuild(docs []Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.docs = make(map[string]*indexedDoc, len(docs))
	idx.docLenCache = make(map[string]int, len(docs))
	idx.df = make(map[string]int)
	idx.totalLen = 0
	for _, d := range docs {
		idx.addLocked(d)
	}
}

// Size returns the number of indexed documents.
func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.docs)
}

// AvgDocLen returns the average document length in tokens, or 0
// for an empty index.
func (idx *Index) AvgDocLen() float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if len(idx.docs) == 0 {
		return 0
	}
	return float64(idx.totalLen) / float64(len(idx.docs))
}

// Search returns the top limit documents matching the query,
// ranked by BM25 score (descending). An empty query returns an
// empty slice.
func (idx *Index) Search(query string, limit int) []Result {
	if limit <= 0 {
		limit = 10
	}
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}
	// Deduplicate query terms so each term's IDF is computed
	// once per document.
	unique := make(map[string]struct{}, len(queryTokens))
	for _, t := range queryTokens {
		unique[t] = struct{}{}
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.docs) == 0 {
		return nil
	}
	N := float64(len(idx.docs))
	avgLen := float64(idx.totalLen) / N

	type scored struct {
		doc   *indexedDoc
		score float64
	}
	scores := make([]scored, 0, len(idx.docs))
	for _, d := range idx.docs {
		score := bm25Score(d, unique, N, avgLen, idx.df)
		if score > 0 {
			scores = append(scores, scored{doc: d, score: score})
		}
	}
	// Stable sort: score descending, then docID ascending for
	// deterministic output across runs.
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		return scores[i].doc.doc.ID < scores[j].doc.doc.ID
	})
	if len(scores) > limit {
		scores = scores[:limit]
	}
	out := make([]Result, len(scores))
	for i, s := range scores {
		out[i] = Result{Document: &s.doc.doc, Score: s.score}
	}
	return out
}

// bm25Score computes the BM25 score for one document given the
// unique query terms, the corpus size N, the average document
// length, and the document-frequency map. Returns 0 if the
// document matches none of the terms.
//
// The formula:
//
//	score(D, Q) = sum over t in Q of:
//	  IDF(t) * (tf(t, D) * (k1 + 1)) /
//	    (tf(t, D) + k1 * (1 - b + b * |D| / avgdl))
//
// where IDF(t) = ln(1 + (N - df(t) + 0.5) / (df(t) + 0.5))
// (the "lucene" smoothed form, bounded below by 0).
func bm25Score(d *indexedDoc, queryTerms map[string]struct{}, n, avgLen float64, df map[string]int) float64 {
	if avgLen <= 0 {
		return 0
	}
	docLen := float64(len(d.tokens))
	if docLen == 0 {
		return 0
	}
	var score float64
	for term := range queryTerms {
		tf, ok := d.tf[term]
		if !ok || tf == 0 {
			continue
		}
		// IDF, smoothed.
		dfTerm := float64(df[term])
		idf := log1Plus(n-dfTerm+0.5, dfTerm+0.5)
		// Length-normalised term frequency.
		numerator := float64(tf) * (bm25K1 + 1)
		denom := float64(tf) + bm25K1*(1-bm25B+bm25B*docLen/avgLen)
		score += idf * numerator / denom
	}
	return score
}

// log1Plus computes ln(1 + (N - df + 0.5) / (df + 0.5)). Pulled
// out as a helper so tests can verify the boundary behaviour
// without re-implementing the formula.
func log1Plus(numeratorMinus, denom float64) float64 {
	if denom <= 0 {
		return 0
	}
	ratio := (numeratorMinus)/denom + 1
	if ratio <= 0 {
		return 0
	}
	return math.Log(ratio)
}

// tokenize splits text into lowercase tokens and bigrams, applies
// a light suffix strip, and returns a flat slice. Unicode letters
// and digits are kept; everything else is a separator. The
// returned slice may contain duplicates (the caller is
// responsible for counting).
func tokenize(text string) []string {
	if text == "" {
		return nil
	}
	// Lowercase + replace non-alphanumeric with spaces.
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	raw := strings.Fields(b.String())
	if len(raw) == 0 {
		return nil
	}
	// Light suffix strip: drop a trailing "s' for crude
	// plural handling. This is not a full Porter stemmer but
	// it improves recall for English content without adding
	// a dependency.
	stemmed := make([]string, 0, len(raw)*2)
	for _, t := range raw {
		if len(t) > 3 && t[len(t)-1] == 's' {
			t = t[:len(t)-1]
		}
		stemmed = append(stemmed, t)
	}
	// Add bigrams: t1 t2, t2 t3, ... Append to a separate
	// buffer, then concatenate, to avoid mutating the slice we
	// are iterating over (which would cause an infinite loop
	// if the underlying array was reallocated and the old
	// pointer still walked forward into a new bigger buffer).
	bigrams := make([]string, 0, len(stemmed)-1)
	for i := 0; i+1 < len(stemmed); i++ {
		bigrams = append(bigrams, stemmed[i]+" "+stemmed[i+1])
	}
	return append(stemmed, bigrams...)
}
