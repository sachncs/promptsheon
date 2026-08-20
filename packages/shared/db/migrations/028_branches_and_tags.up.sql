-- 028_branches_and_tags.up.sql
-- Phase 1.2: branches (movable refs) and tags (frozen refs).

CREATE TABLE branches (
    id TEXT PRIMARY KEY,
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    head_commit_oid TEXT,
    is_protected INTEGER NOT NULL DEFAULT 0
        CHECK (is_protected IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (repository_id, name)
);

CREATE INDEX idx_branches_repository ON branches(repository_id);

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    commit_oid TEXT NOT NULL,
    message TEXT,
    tagger_id TEXT NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (repository_id, name)
);

CREATE INDEX idx_tags_repository ON tags(repository_id);
CREATE INDEX idx_tags_commit ON tags(commit_oid);
