-- 009 down: vault_state is a singleton. Dropping it is safe.
-- Existing ciphertexts are AES-GCM ciphertexts of the wrapped
-- data key; without vault_state the Provider falls back to
-- GenerateDataKey on next access. No data loss beyond the
-- persisted wrapped blob.

DROP TABLE vault_state;
