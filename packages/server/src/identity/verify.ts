import { principalFromRequest, type Principal } from '../policy/principal.js';
import { sha256Hex, verifyApiKey, type ApiKeyVerification } from './apikey.js';
import { verifySVID, type SvidPayload } from './svid.js';
import { createHash } from 'node:crypto';

/**
 * Identifies the credential scheme carried in the
 * `Authorization` header. The route layer (AG-4.5) looks up the
 * stored credential row and hands the result to the verifier.
 */
export type IdentityKind = 'apikey' | 'svid' | 'none';

export interface IdentityResolution {
  kind: IdentityKind;
  principal: Principal;
  /** Optional scope tag from the credential (apikeys only for v1). */
  scope?: string;
  /** SVID payload (svid only). */
  payload?: SvidPayload;
}

export interface VerifyPrincipalInput {
  /** The raw `Authorization` header value. */
  authorization: string | undefined;
  /**
   * All other relevant headers — X-User-Id, X-Org-Id, X-Agent-Id,
   * X-Agent-Classification. Same shape as the principalFromRequest
   * helper.
   */
  headers: Record<string, string | undefined>;
  /**
   * The stored credential row to verify against. For apikeys: a
   * `verifyApiKey` argument. For SVIDs: the public key (PEM) the
   * operator registered for this org.
   */
  apikey?: ApiKeyVerification | undefined;
  svidPublicKey?: string | undefined;
}

/**
 * Top-level verifier: reads the request headers, picks the scheme,
 * calls the right verifier, and returns a typed `Principal` ready
 * for the Cedar gate. Returns null when no usable credential is
 * present, so the route can 401.
 */
export function verifyPrincipal(input: VerifyPrincipalInput): IdentityResolution | null {
  const auth = input.authorization;
  if (typeof auth !== 'string' || auth.length === 0) {
    // No Authorization header — fall through to the principal
    // extraction pipeline (which may still produce a principal
    // from X-User-Id etc.). The system-actor override is the
    // caller's responsibility (see policy/principal.ts).
    return resolveFromHeaders(input);
  }
  if (auth.startsWith('Bearer ')) {
    const token = auth.slice('Bearer '.length);
    if (input.apikey && verifyApiKey(token, input.apikey)) {
      const principal: Principal = {
        type: 'User',
        id: input.apikey.agentId,
        orgId: input.apikey.orgId,
        role: 'viewer',
      };
      return { kind: 'apikey', principal, scope: input.apikey.scope };
    }
    // Bearer token didn't match the stored apikey — fall through
    // to header-based extraction (legacy X-User-Id path).
    return resolveFromHeaders(input);
  }
  if (auth.startsWith('SVID ')) {
    const token = auth.slice('SVID '.length);
    if (typeof input.svidPublicKey === 'string') {
      const v = verifySVID(token, input.svidPublicKey);
      if (v !== null) {
        const principal: Principal = {
          type: 'Agent',
          id: v.payload.sub,
          orgId: v.payload.org,
          classification: v.payload.cls,
        };
        return { kind: 'svid', principal, scope: v.payload.scope.join(','), payload: v.payload };
      }
    }
    return null;
  }
  return resolveFromHeaders(input);
}

function resolveFromHeaders(input: VerifyPrincipalInput): IdentityResolution | null {
  const principal = principalFromRequest(input.headers);
  if (principal === null) return null;
  return { kind: 'none', principal };
}

/**
 * Convenience for the route layer: compute the SHA-256 of a
 * bearer token. The apikey.credential column stores this hash,
 * not the raw token, so the verifier can compare in constant time.
 */
export function hashBearer(token: string): string {
  return sha256Hex(token);
}

/**
 * Convenience: compute the digest of an SVID for revocations /
 * audit rows.
 */
export function digestSvid(token: string): string {
  return createHash('sha256').update(token).digest('hex');
}