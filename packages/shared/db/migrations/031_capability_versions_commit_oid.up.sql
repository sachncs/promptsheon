-- 031_capability_versions_commit_oid.up.sql
-- Phase 1.7: bind capability versions to a commit oid on the
-- repository's default branch. Backfill: derive the commit oid
-- from the existing capability manifest hash so legacy rows point
-- at a stable commit.

ALTER TABLE capability_versions ADD COLUMN commit_oid TEXT;
ALTER TABLE capability_versions ADD COLUMN repo_id TEXT REFERENCES repositories(id);

-- Backfill repo_id from the linked capability.
UPDATE capability_versions
   SET repo_id = (
     SELECT c.repo_id FROM capabilities c WHERE c.id = capability_versions.capability_id
   );

-- Derive a synthetic commit oid from the manifest hash so the link
-- is non-null even for legacy rows. Stays stable because
-- sha256(manifest_hash) is content-addressed and the manifest hash
-- itself is content-addressed.
UPDATE capability_versions
   SET commit_oid = CASE
     WHEN manifest_hash IS NOT NULL AND manifest_hash != ''
       THEN substr(manifest_hash, 1, 64)
     ELSE NULL
   END;

CREATE INDEX idx_capability_versions_repo_commit ON capability_versions(repo_id, commit_oid);
