import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  createHash,
  createPublicKey,
  createPrivateKey,
  generateKeyPairSync,
  sign,
  verify,
} from 'node:crypto';
import { fingerprintSpki, generateEd25519KeyPair } from '../src/repos/signing-key.js';
import { signPayload, signedMessage } from '../src/routes/signing.js';

function signEd(privateKeyPem: string, msg: Buffer): Buffer {
  return sign(null, msg, createPrivateKey(privateKeyPem));
}

function verifySig(publicKeyPem: string, msg: Buffer, sig: Buffer): boolean {
  return verify(null, msg, createPublicKey(publicKeyPem), sig);
}

describe('operator-managed signing keys', () => {
  it('fingerprintSpki matches sha256 of the SPKI DER bytes', () => {
    const { publicKey } = generateKeyPairSync('ed25519');
    const pem = publicKey.export({ format: 'pem', type: 'spki' }).toString();
    const fp = fingerprintSpki(pem);
    const keyObject = createPublicKey(pem);
    const der = keyObject.export({ format: 'der', type: 'spki' });
    const expected = createHash('sha256').update(der).digest('hex');
    expect(fp).toBe(expected);
  });

  it('different keys produce different fingerprints', () => {
    const a = generateKeyPairSync('ed25519');
    const b = generateKeyPairSync('ed25519');
    const fpA = fingerprintSpki(a.publicKey.export({ format: 'pem', type: 'spki' }).toString());
    const fpB = fingerprintSpki(b.publicKey.export({ format: 'pem', type: 'spki' }).toString());
    expect(fpA).not.toBe(fpB);
  });

  it('signedMessage is sha256(oid|ref|approver|timestamp)', () => {
    const msg = signedMessage({
      commitOid: 'a'.repeat(64),
      ref: 'main',
      approverId: 'u1',
      timestamp: '2026-08-20T16:00:00Z',
    });
    const expected = createHash('sha256')
      .update(`${'a'.repeat(64)}\x1fmain\x1fu1\x1f2026-08-20T16:00:00Z`)
      .digest();
    expect(msg.equals(expected)).toBe(true);
  });

  it('verify accepts an ed25519 signature produced off-box', () => {
    const { publicKeyPem, privateKeyPem } = generateEd25519KeyPair();
    const commitOid = 'b'.repeat(64);
    const approverId = 'u1';
    const timestamp = new Date().toISOString();
    const msg = signedMessage({ commitOid, ref: 'main', approverId, timestamp });
    const sig = signEd(privateKeyPem, msg);
    const valid = verifySig(publicKeyPem, msg, sig);
    expect(valid).toBe(true);
  });

  it('verify rejects a tampered message', () => {
    const { publicKeyPem, privateKeyPem } = generateEd25519KeyPair();
    const commitOid = 'c'.repeat(64);
    const timestamp = new Date().toISOString();
    const msg = signedMessage({ commitOid, ref: 'main', approverId: 'u1', timestamp });
    const sig = signEd(privateKeyPem, msg);
    const tampered = signedMessage({
      commitOid: 'd'.repeat(64),
      ref: 'main',
      approverId: 'u1',
      timestamp,
    });
    const valid = verifySig(publicKeyPem, tampered, sig);
    expect(valid).toBe(false);
  });

  it('verify rejects a signature from a different key', () => {
    const a = generateEd25519KeyPair();
    const b = generateEd25519KeyPair();
    const commitOid = 'e'.repeat(64);
    const timestamp = new Date().toISOString();
    const msg = signedMessage({ commitOid, ref: 'main', approverId: 'u1', timestamp });
    const sig = signEd(b.privateKeyPem, msg);
    const valid = verifySig(a.publicKeyPem, msg, sig);
    expect(valid).toBe(false);
  });

  it('signPayload and signedMessage produce different bytes for the same input', () => {
    const input = {
      commitOid: 'f'.repeat(64),
      ref: 'main',
      approverId: 'u1',
      timestamp: new Date().toISOString(),
    };
    const a = signedMessage(input);
    const b = signPayload(input);
    // signedMessage is sha256(canonicalString); signPayload is a
    // pre-image built for sha256-canonical-json payloads. They're
    // not bit-equal. Both are deterministic and verifiable in
    // production with the canonical message.
    expect(Buffer.isBuffer(a)).toBe(true);
    expect(Buffer.isBuffer(b)).toBe(true);
    expect(a.equals(b)).toBe(false);
  });

  it('keys survive a sha256 round-trip for SPKI fingerprinting', () => {
    const kp = generateKeyPairSync('ed25519');
    const pem = kp.publicKey.export({ format: 'pem', type: 'spki' }).toString();
    const f1 = fingerprintSpki(pem);
    const f2 = fingerprintSpki(pem);
    expect(f1).toBe(f2);
    expect(f1.length).toBe(32);
  });
});
