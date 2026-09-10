-- 029_repo_blobs.up.sql
-- Phase 1.3: file-backed content-addressed store per repository.
-- A blob is identified by sha256(content). A tree is a
-- canonicalised JSON object mapping path -> blob_oid.

CREATE TABLE repo_blobs (
    oid TEXT PRIMARY KEY,
    size INTEGER NOT NULL,
    content BLOB NOT NULL
);

CREATE TABLE repo_trees (
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    ref TEXT NOT NULL,
    path TEXT NOT NULL,
    blob_oid TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repository_id, ref, path)
);

CREATE INDEX idx_repo_trees_ref ON repo_trees(repository_id, ref);

-- A pinning table: which (repository, ref, tree-oid) triples have
-- been promoted to a commit. Used to compute authoritative tree
-- hashes without scanning the entries.
CREATE TABLE repo_pinned_trees (
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    ref TEXT NOT NULL,
    tree_oid TEXT NOT NULL,
    committed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repository_id, ref)
);
