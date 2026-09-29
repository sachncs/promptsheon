import { createHash, createPublicKey, verify } from 'node:crypto';
import type { Release } from '@promptsheon/shared';

export function releaseManifestHash(manifest: string): string {
  return createHash('sha256').update(manifest).digest('hex');
}

export function signedReleaseMessage(input: { releaseId: string; manifestHash: string; timestamp: string }): Buffer {
  return createHash('sha256')
    .update(`${input.releaseId}\x1f${input.manifestHash}\x1f${input.timestamp}`)
    .digest();
}

export function verifyReleaseSignature(release: Release, publicKeyPem: string): boolean {
  if (!release.signature || !release.signedAt) return false;
  try {
    return verify(
      null,
      signedReleaseMessage({ releaseId: release.id, manifestHash: releaseManifestHash(release.manifest), timestamp: release.signedAt }),
      createPublicKey(publicKeyPem),
      Buffer.from(release.signature, 'base64'),
    );
  } catch {
    return false;
  }
}
