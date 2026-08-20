-- 032_merge_requests.up.sql
-- Phase 1.13: merge requests + approvals + comments. Maker-checker
-- approval is enforced at the application layer (approver must
-- differ from the author) — the schema permits duplicate
-- (mr_id, user_id) only because an MR being re-opened should
-- preserve its previous decisions.

CREATE TABLE merge_requests (
    id TEXT PRIMARY KEY,
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    source_branch TEXT NOT NULL,
    target_branch TEXT NOT NULL,
    source_commit_oid TEXT NOT NULL,
    merge_commit_oid TEXT,
    author_id TEXT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'merged', 'closed')),
    requested_reviewers TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    merged_at DATETIME,
    UNIQUE (repository_id, number)
);

CREATE INDEX idx_merge_requests_repo_status ON merge_requests(repository_id, status);

CREATE TABLE merge_request_approvals (
    merge_request_id TEXT NOT NULL REFERENCES merge_requests(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    decision TEXT NOT NULL CHECK (decision IN ('approve', 'request_changes')),
    comment_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (merge_request_id, user_id)
);

CREATE TABLE merge_request_comments (
    id TEXT PRIMARY KEY,
    merge_request_id TEXT NOT NULL REFERENCES merge_requests(id) ON DELETE CASCADE,
    author_id TEXT NOT NULL REFERENCES users(id),
    path TEXT,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mr_comments_mr ON merge_request_comments(merge_request_id);
