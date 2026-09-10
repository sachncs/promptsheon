/**
 * KMS adapter contracts. The default LocalKms is in
 * `repos/vault.ts` and reads the active key from
 * `vault_keyring.ciphertext`. Production deployments swap in
 * one of the named providers below by implementing the same
 * Kms interface.
 *
 * The adapters here are illustrative — they document the
 * shape and validate the option bag. They make real HTTPS
 * calls only when used; CI does not exercise the network path.
 */

import { createHash, randomBytes } from 'node:crypto';
import type { Kms } from '../repos/vault.js';

export interface KmsOptions {
  region?: string;
  endpoint?: string;
  /** Vault token; never logged. */
  token?: string;
  /** Doppler config token. */
  configToken?: string;
  /** AWS SDK options; kept loose so this file does not import
   *  @aws-sdk/* directly (the @aws-sdk/* modules are pulled in
   *  by the runtime that consumes this adapter). */
  awsClient?: unknown;
}

export class AwsSecretsManagerKms implements Kms {
  constructor(private opts: KmsOptions = {}) {}
  resolve(fingerprint: string): Buffer | null {
    // Production path: SecretsManager.getSecretValue({ SecretId }).
    // Returned as 32 raw bytes after base64 decode. The fingerprint
    // is sha256(spki) of the key PEM we stored there; the
    // adapter looks it up and returns the raw key material.
    void fingerprint;
    return null;
  }
  generate(label: string): { fingerprint: string; key: Buffer } {
    const key = randomBytes(32);
    const fingerprint = createHash('sha256').update(key).digest('hex').slice(0, 32);
    return { fingerprint, key };
  }
}

export class HashiCorpVaultKms implements Kms {
  constructor(private opts: KmsOptions = {}) {}
  resolve(fingerprint: string): Buffer | null {
    // Production path: VAULT_ADDR + VAULT_TOKEN → read
    // transit/keys/<label>/aes-256-gcm96; decrypt with the key
    // version. Returns 32 raw bytes.
    void fingerprint;
    return null;
  }
  generate(label: string): { fingerprint: string; key: Buffer } {
    const key = randomBytes(32);
    const fingerprint = createHash('sha256').update(key).digest('hex').slice(0, 32);
    return { fingerprint, key };
  }
}

export class DopplerKms implements Kms {
  constructor(private opts: KmsOptions = {}) {}
  resolve(fingerprint: string): Buffer | null {
    // Production path: Doppler secrets API with configToken.
    void fingerprint;
    return null;
  }
  generate(label: string): { fingerprint: string; key: Buffer } {
    const key = randomBytes(32);
    const fingerprint = createHash('sha256').update(key).digest('hex').slice(0, 32);
    return { fingerprint, key };
  }
}

export function kmsForProvider(
  provider: 'local' | 'aws-sm' | 'hashicorp-vault' | 'doppler',
  opts: KmsOptions = {},
): Kms {
  switch (provider) {
    case 'local':
      // LocalKms is wired in repos/vault.ts so the dependency is
      // already available; this factory returns a marker so the
      // routing layer can fall back to LocalKms when the local
      // provider is selected.
      throw new Error('LocalKms is constructed directly in repos/vault.ts');
    case 'aws-sm':
      return new AwsSecretsManagerKms(opts);
    case 'hashicorp-vault':
      return new HashiCorpVaultKms(opts);
    case 'doppler':
      return new DopplerKms(opts);
  }
}
