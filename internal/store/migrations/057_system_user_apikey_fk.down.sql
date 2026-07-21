-- Migration 057 down: remove the seeded system user. The FK on
-- api_keys.user_id was never added in this migration's forward
-- direction; the seed is reversible without affecting FK
-- integrity.
DELETE FROM users WHERE id = 'api';
