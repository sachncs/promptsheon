-- 009: vault_state (singleton) for KMS-wrapped data key.
--
-- SEC-10a: persist the ciphertextBlob returned by
-- GenerateDataKey. The Provider reads it on cache miss and calls
-- KMSClient.Decrypt to unwrap. This lets a process survive KMS
-- rotations: when the underlying AWS KMS key is re-encrypted, the
-- next Decrypt against the persisted ciphertextBlob reflects the
-- new key.

CREATE TABLE vault_state (
    id               INTEGER PRIMARY KEY CHECK (id = 0),
    kms_key_id       TEXT NOT NULL,
    wrapped_data_key BLOB NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
