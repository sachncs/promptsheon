import { describe, expect, it } from 'vitest';
import { generateKeyPairSync, sign } from 'node:crypto';
import { releaseManifestHash, signedReleaseMessage, verifyReleaseSignature } from '../../src/application/release-signing.js';
import type { Release } from '@promptsheon/shared';

const baseRelease: Release = {
  id: 'release-1', capabilityId: 'cap-1', capabilityVersion: 1, capabilityVersionId: null,
  manifest: '{"id":"manifest-1"}', environment: 'prod', status: 'review', approvedBy: '',
  replacesReleaseId: null, createdAt: '2026-01-01T00:00:00.000Z', createdBy: 'operator',
  activatedAt: null, canaryPercent: 0,
};

describe('release signing', () => {
  it('verifies the signed manifest identity and rejects tampering', () => {
    const { privateKey, publicKey } = generateKeyPairSync('ed25519');
    const signedAt = '2026-01-01T00:01:00.000Z';
    const signature = sign(null, signedReleaseMessage({ releaseId: baseRelease.id, manifestHash: releaseManifestHash(baseRelease.manifest), timestamp: signedAt }), privateKey).toString('base64');
    const release = { ...baseRelease, signature, signedAt };
    const publicKeyPem = publicKey.export({ format: 'pem', type: 'spki' }).toString();

    expect(verifyReleaseSignature(release, publicKeyPem)).toBe(true);
    expect(verifyReleaseSignature({ ...release, manifest: '{"id":"tampered"}' }, publicKeyPem)).toBe(false);
  });
});
