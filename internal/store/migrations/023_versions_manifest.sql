-- Migration 023: introduce capability_versions.manifest (CAS composition).
--
-- ADR-0010 makes the Version's bundle of 12 leaf artifacts into a single
-- Manifest JSON value, content-addressed by SHA-256. This migration adds
-- the new column alongside the existing per-artifact columns so the
-- transition is forward-only and reversible: new code writes to `manifest`
-- and reads from it, legacy code (during the transition window) continues
-- to use the per-artifact columns. A subsequent migration will drop the
-- per-artifact columns once the codebase is fully migrated.

ALTER TABLE capability_versions ADD COLUMN manifest TEXT NOT NULL DEFAULT '{}';
ALTER TABLE capability_versions ADD COLUMN manifest_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_versions_manifest_hash ON capability_versions(manifest_hash);
