# Deferred Work

This file tracks acceptance-criteria items from `phase-1-security.md`,
`phase-2-migrations.md`, and the schema architecture review that are
**structurally complete but not yet exercised live** in the
deployed path, plus items the maintainer has explicitly deferred.

## Format

Each entry has:
- **Item**: the ID and short title.
- **Spec wording**: the literal acceptance from the todo file.
- **Shipped state**: what the codebase actually does.
- **Why deferred**: structural gap, scope, or maintainer decision.
- **Work to pick it up**: what the next change would look like.

---

## SEC-10a — KMS re-reads wrapped blob on cache miss ✅ DONE

### Spec wording
> Re-encrypting a secret with a new KMS key is reflected on the
> next read; decrypt still works after the wrapped blob rotates.

### Shipped state
`KMSClient` interface gained `Decrypt`. `AWSKMSClient` wraps
`kms:Decrypt`. Migration `009_vault_state.up.sql` creates a
singleton `vault_state` table holding `(kms_key_id,
wrapped_data_key, created_at, updated_at)`. `Provider` now
persists the CiphertextBlob returned by
`GenerateDataKeyWithCiphertextBlob` and reads it on cache
miss via `Decrypt`. The plaintext cache is an LRU of size 16
keyed by `sha256(wrapped_data_key)`. On rotation, the wrapped
blob changes, the LRU key changes, and the next read Decrypts
the new blob.

Tests added in `internal/vault/kmsbyok/provider_test.go`:
- `TestProviderPersistsWrappedBlob` — first call writes
  vault_state; second call hits LRU.
- `TestProviderReflectsRotatedBlob` — rotation changes the
  wrapped blob; next read returns new plaintext.
- `TestProviderDecryptFailureDoesNotPoisonLRU` — failed Decrypt
  surfaces, no negative caching.
- `TestProviderLRUSize16` — LRU stays at 16 entries.

---

## Maintenance

This file is reviewed at the end of every phase. Items are
removed (or migrated to `todo/<phase>.md`) once their deferred
work ships.

---

## Maintenance

This file is reviewed at the end of every phase. Items are
removed (or migrated to `todo/<phase>.md`) once their deferred
work ships.
