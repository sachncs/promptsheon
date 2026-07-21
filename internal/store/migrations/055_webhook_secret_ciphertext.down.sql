-- Migration 055 down: drop the ciphertext column.
ALTER TABLE webhook_endpoints DROP COLUMN secret_ciphertext;
