# ADR 0002: Use BM25 instead of vector search for prompt retrieval

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

We want to find "prompts similar to this one" and to power the `?search=` query on `GET /api/v1/prompts`. There are two well-known families of approaches: lexical (BM25, TF-IDF) and semantic (dense vector search using embeddings).

Vector search captures meaning ("research agent" matches "literature review agent") but requires an embedding model, an index, and external dependencies (`pgvector`, FAISS, or a managed service). For a single-binary deployable, that is a heavy cost for a feature used as a UX nicety, not a core capability.

## Decision

We use a BM25 index implemented in-process in `internal/search/bm25.go`. The standard Okapi BM25 hyperparameters are used: `k1=1.2`, `b=0.75`. Tokens are unigrams and bigrams with a light suffix strip for crude plural handling, and per-document length normalisation is applied.

The index is rebuilt on demand and is held in memory. There is no persistent index file.

## Options considered

1. **BM25 in-process (chosen).** Fast, deterministic, zero external dependencies, easy to test. Good enough for prompts, which are short English text.
2. **Vector search (FAISS / pgvector).** Better semantic match. Requires a model, an index, and a server. Out of scope for a single-binary release.
3. **Hybrid BM25 + vector.** Best of both, but doubles the moving parts. Punt to v2.
4. **Simple `LIKE '%query%'`.** Trivially correct, terrible ranking.

## Consequences

Positive:

- Zero external dependencies. The index is part of the binary.
- Deterministic ranking: the same query against the same corpus always returns the same scores.
- Cheap to test. The package ships a unit test that builds an index from canned input.
- No embedding model means no download step on first use.

Negative:

- Lexical only. "research agent" will not match "literature review agent" unless the words overlap.
- The in-memory index is rebuilt on every server start. Acceptable up to tens of thousands of prompts; becomes a problem at much higher counts.
- Bigrams help with phrasing but do not generalise to morphologically rich languages.

When semantic search becomes a hard requirement, a hybrid approach is the natural next step (see [Algorithms — BM25](../algorithms.md#bm25)).

## References

- `internal/search/bm25.go` — index implementation
- `internal/search/manager.go` — thread-safe API used by the HTTP handlers
- [Algorithms — BM25](../algorithms.md#bm25) — details and pseudocode
