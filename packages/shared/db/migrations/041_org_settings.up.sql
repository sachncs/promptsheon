-- 041_org_settings.up.sql
-- Phase 3 polish: per-organisation data residency + per-tenant
-- encryption-at-rest toggle. Residency records which region the
-- org's data should live in; encryptionToggle controls whether
-- the vault envelope is enabled (default ON for production
-- tenants, OFF for the dev/local install).

ALTER TABLE orgs ADD COLUMN residency TEXT NOT NULL DEFAULT 'local'
  CHECK (residency IN ('local', 'us', 'eu', 'ap', 'sa', 'me', 'af'));
ALTER TABLE orgs ADD COLUMN encryption_at_rest INTEGER NOT NULL DEFAULT 1
  CHECK (encryption_at_rest IN (0, 1));
ALTER TABLE orgs ADD COLUMN kms_provider TEXT NOT NULL DEFAULT 'local'
  CHECK (kms_provider IN ('local', 'aws-sm', 'hashicorp-vault', 'doppler'));
