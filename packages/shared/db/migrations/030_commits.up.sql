-- 030_commits.up.sql
-- Phase 1.5: content-addressed commit objects keyed by tree-oid +
-- parents + author + message + timestamp. A commit oid is
-- sha256(canonicalJson(payload)) where canonicalJson sorts
-- parents and the other fields are stable.

CREATE TABLE repo_commits (
    oid TEXT PRIMARY KEY,
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    ref TEXT NOT NULL,
    tree_oid TEXT NOT NULL,
    parents TEXT NOT NULL DEFAULT '[]',
    author_id TEXT NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    signature TEXT,
    signed_key_id TEXT,
    signed_at TEXT
);

CREATE INDEX idx_repo_commits_repo_ref ON repo_commits(repository_id, ref);
CREATE INDEX idx_repo_commits_tree ON repo_commits(tree_oid);
