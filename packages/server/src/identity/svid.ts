import { createPrivateKey, createPublicKey, sign, verify, KeyObject } from 'node:crypto';

/**
 * Short-lived workload identity (SPIFFE-style) for an Agent.
 * The SVID is a signed JSON payload + ed25519 signature. The
 * verifier is pure: given the SVID token and a public key, it
 * returns the payload or null (bad signature / expired / wrong
 * format). No IO, no DB.
 *
 * Token format: `svid_<base64url(payload)>.<base64url(signature)>`.
 * The base64url alphabet is URL-safe and avoids padding so the
 * token survives an `Authorization: SVID ...` header without
 * quoting.
 *
 * Wire shape (the signed payload):
 *   {
 *     "iss": "<promptsheon>",      // issuer
 *     "sub": "<agent-id>",          // subject (the agent)
 *     "org": "<org-id>",            // org scope
 *     "iat": <unix-seconds>,        // issued-at
 *     "exp": <unix-seconds>,        // expiry
 *     "nbf": <unix-seconds>,        // not-before
 *     "scope": ["..."],             // scopes (gateway, memory, ...)
 *     "cls": "<classification>"     // data sensitivity
 *   }
 *
 * The verifier is timing-safe: it uses `crypto.verify` which
 * itself is constant-time for the underlying ed25519 primitive.
 */

export interface SvidPayload {
  iss: string;
  sub: string;
  org: string;
  iat: number;
  exp: number;
  nbf: number;
  scope: string[];
  cls: string;
}

export interface MintSvidOptions {
  agentId: string;
  orgId: string;
  /** Ed25519 private key, PEM-encoded (PKCS8) or a KeyObject. */
  signingKey: string | KeyObject;
  /** Time-to-live in seconds. Defaults to 15 minutes. */
  ttlSeconds?: number;
  scope?: string[];
  classification?: string;
  /** Override the issued-at time. */
  issuedAt?: Date;
}

export interface SvidVerification {
  payload: SvidPayload;
  /** Hex of the SHA-256 of the canonical payload. */
  payloadDigest: string;
}

const ISSUER = 'promptsheon';
const SAFE_AGENT_ID = /^[A-Za-z0-9_-]{1,128}$/;

export function mintSVID(opts: MintSvidOptions): string {
  if (!SAFE_AGENT_ID.test(opts.agentId)) {
    throw new Error(`invalid agentId: ${opts.agentId}`);
  }
  const now = opts.issuedAt ?? new Date();
  const ttl = opts.ttlSeconds ?? 15 * 60;
  const payload: SvidPayload = {
    iss: ISSUER,
    sub: opts.agentId,
    org: opts.orgId,
    iat: Math.floor(now.getTime() / 1000),
    nbf: Math.floor(now.getTime() / 1000),
    exp: Math.floor(now.getTime() / 1000) + ttl,
    scope: opts.scope ?? ['gateway', 'memory', 'tool'],
    cls: opts.classification ?? 'internal',
  };
  const keyObj =
    typeof opts.signingKey === 'string' ? createPrivateKey(opts.signingKey) : opts.signingKey;
  const payloadBytes = Buffer.from(JSON.stringify(payload), 'utf8');
  const sig = sign(null, payloadBytes, keyObj);
  return `svid_${payloadBytes.toString('base64url')}.${sig.toString('base64url')}`;
}

function parseSvid(token: string): { payload: Buffer; signature: Buffer } | null {
  if (typeof token !== 'string' || token.length === 0) return null;
  if (!token.startsWith('svid_')) return null;
  const stripped = token.slice('svid_'.length);
  const dot = stripped.indexOf('.');
  if (dot < 0) return null;
  const payloadB64 = stripped.slice(0, dot);
  const sigB64 = stripped.slice(dot + 1);
  if (payloadB64.length === 0 || sigB64.length === 0) return null;
  try {
    return {
      payload: Buffer.from(payloadB64, 'base64url'),
      signature: Buffer.from(sigB64, 'base64url'),
    };
  } catch {
    return null;
  }
}

/**
 * Verify an SVID token against a public key. Returns the
 * verification (payload + digest) on success, or null on any
 * failure. Timing-safe: uses `crypto.verify` for the signature
 * check and a wall-clock comparison for the time window.
 */
export function verifySVID(
  token: string,
  publicKey: string | KeyObject,
  options: { now?: Date } = {},
): SvidVerification | null {
  const parsed = parseSvid(token);
  if (parsed === null) return null;
  let payload: SvidPayload;
  try {
    payload = JSON.parse(parsed.payload.toString('utf8')) as SvidPayload;
  } catch {
    return null;
  }
  if (payload.iss !== ISSUER) return null;
  if (typeof payload.sub !== 'string' || payload.sub.length === 0) return null;
  if (!Array.isArray(payload.scope)) return null;
  if (typeof payload.iat !== 'number' || typeof payload.exp !== 'number' || typeof payload.nbf !== 'number') {
    return null;
  }

  const keyObj =
    typeof publicKey === 'string' ? createPublicKey(publicKey) : publicKey;
  let valid = false;
  try {
    valid = verify(null, parsed.payload, {
      key: keyObj,
      dsaEncoding: 'ieee-p1363',
    }, parsed.signature);
  } catch {
    return null;
  }
  if (!valid) return null;

  const now = options.now ?? new Date();
  const nowSec = Math.floor(now.getTime() / 1000);
  if (nowSec < payload.nbf) return null;
  if (nowSec >= payload.exp) return null;

  return {
    payload,
    payloadDigest: parsed.payload.toString('hex'),
  };
}

/**
 * Convenience: generate a throwaway ed25519 keypair for tests +
 * dev. Production uses the per-org signing key from
 * `signing_keys` (see repos/signing-key.ts).
 */
export function generateSvidSigningKey(): {
  publicKey: string;
  privateKey: string;
} {
  // Inline import of generateKeyPairSync to avoid pulling it into
  // the production bundle. Tests use this helper; production goes
  // through SigningKeyRepo.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { generateKeyPairSync } = require('node:crypto') as typeof import('node:crypto');
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  return {
    publicKey: publicKey.export({ format: 'pem', type: 'spki' }).toString(),
    privateKey: privateKey.export({ format: 'pem', type: 'pkcs8' }).toString(),
  };
}