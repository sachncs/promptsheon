import { createHash, randomBytes, timingSafeEqual } from 'node:crypto';

/**
 * Long-lived API-key credential for an Agent. The raw token is
 * shown to the agent exactly once at mint time and never
 * persisted; we only store sha256(token) so a database leak
 * doesn't leak credentials.
 *
 * Token format: `psk_<agent-id>_<32 random hex>` — long enough
 * that brute-forcing the suffix is infeasible and the prefix
 * makes the token greppable in logs.
 *
 * The verifier is pure: no IO, no DB. The caller (AG-4.5 route)
 * looks up the row by sha256(credential) and checks expiry +
 * revoked_at.
 */

const TOKEN_PREFIX = 'psk_';

export interface MintApiKeyOptions {
  agentId: string;
  orgId: string;
  /** ISO 8601 timestamp. Defaults to now. */
  issuedAt?: string;
  /** Days from issuedAt until expiry. Defaults to 30. */
  ttlDays?: number;
  /** Scope tag persisted alongside the credential. */
  scope?: string;
}

export interface ApiKeyMaterial {
  /** Raw token. Shown to the agent exactly once at mint time. */
  token: string;
  /** sha256(token) hex. Persist this in agent_identities.credential. */
  hash: string;
  /** ISO 8601 timestamp. */
  issuedAt: string;
  /** ISO 8601 timestamp. */
  expiresAt: string;
  agentId: string;
  orgId: string;
  scope: string;
}

export interface ApiKeyVerification {
  hash: string;
  agentId: string;
  orgId: string;
  scope: string;
  expiresAt: string;
  revokedAt: string | null;
}

const SAFE_AGENT_ID = /^[A-Za-z0-9_-]{1,128}$/;

export function mintApiKey(opts: MintApiKeyOptions): ApiKeyMaterial {
  if (!SAFE_AGENT_ID.test(opts.agentId)) {
    throw new Error(`invalid agentId: ${opts.agentId}`);
  }
  const issuedAt = opts.issuedAt ?? new Date().toISOString();
  const expiresAt = new Date(
    Date.parse(issuedAt) + (opts.ttlDays ?? 30) * 86_400_000,
  ).toISOString();
  const suffix = randomBytes(32).toString('hex');
  const token = `${TOKEN_PREFIX}${opts.agentId}_${suffix}`;
  const hash = sha256Hex(token);
  return {
    token,
    hash,
    issuedAt,
    expiresAt,
    agentId: opts.agentId,
    orgId: opts.orgId,
    scope: opts.scope ?? '{}',
  };
}

export function sha256Hex(input: string): string {
  return createHash('sha256').update(input).digest('hex');
}

/**
 * Constant-time string compare. Defends against timing oracles
 * that could let an attacker discover the hash prefix by timing
 * the comparison.
 */
export function constantTimeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  const aBuf = Buffer.from(a);
  const bBuf = Buffer.from(b);
  return timingSafeEqual(aBuf, bBuf);
}

/**
 * Verify a raw token against a stored credential row. Returns
 * null on any failure (wrong hash, expired, revoked). Pure:
 * no IO, no log noise; the caller handles the 401.
 */
export function verifyApiKey(
  token: string,
  stored: ApiKeyVerification,
): boolean {
  if (token.length === 0) return false;
  const expected = stored.hash;
  const actual = sha256Hex(token);
  if (!constantTimeEqual(expected, actual)) return false;
  if (Date.parse(stored.expiresAt) < Date.now()) return false;
  if (stored.revokedAt !== null) return false;
  return true;
}