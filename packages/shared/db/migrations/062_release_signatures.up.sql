ALTER TABLE releases ADD COLUMN release_signature TEXT;
ALTER TABLE releases ADD COLUMN signed_key_id TEXT;
ALTER TABLE releases ADD COLUMN signed_at TEXT;

CREATE INDEX idx_releases_signed_key ON releases(signed_key_id);
