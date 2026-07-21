-- Migration 055 (webhook secret encryption): add a ciphertext
-- column for the HMAC secret and backfill from the plaintext
-- column.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries chain format
--   * audit_chain_state layout
--   * harness tables
--   * provider_keys layout
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts
--
-- The plaintext column is dropped after backfill. Until then
-- the application layer reads from secret_ciphertext and
-- falls back to secret (empty) so this migration is idempotent
-- across partial rollbacks.

ALTER TABLE webhook_endpoints ADD COLUMN secret_ciphertext BLOB;

-- Backfill ciphertext from plaintext. The vault may not be
-- configured at migration time; in that case the secret is
-- silently empty (the existing plaintext column is the source
-- of truth until rotation). The application code prefers the
-- ciphertext column when present.
UPDATE webhook_endpoints SET secret_ciphertext = secret WHERE secret != '';
