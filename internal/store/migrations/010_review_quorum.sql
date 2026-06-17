-- Migration 010: Add quorum support to reviews.
ALTER TABLE reviews ADD COLUMN quorum_required INTEGER NOT NULL DEFAULT 1;
ALTER TABLE reviews ADD COLUMN approvals_count INTEGER NOT NULL DEFAULT 0;
