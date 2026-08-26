import * as cedar from '@cedar-policy/cedar-wasm';

/**
 * The Cedar principal the authorizer evaluates against.
 *
 * - `User` — a human in an org, identified by an X-User-Id or
 *   `Authorization: Bearer` token. The `role` field is the user's
 *   promptsheon role and gates which Cedar policy clauses match.
 * - `Agent` — a workload identity (SVID or API key). The
 *   `classification` field is the data sensitivity clearance
 *   the agent runs at; cross-agent actions require target.classification
 *   ≤ agent.classification (enforced at the gateway, not in Cedar).
 * - `System` — the bootstrap / replication / internal-actor
 *   principal. Only honored when `PROMPTSHEON_ALLOW_SYSTEM_ACTOR=true`.
 *
 * Cedar's `entity { ... }` types must be declared in the schema;
 * see PROMPTSHEON_SCHEMA in `./authorizer.js`.
 */
export type Principal =
  | { type: 'User'; id: string; orgId: string; role: string }
  | { type: 'Agent'; id: string; orgId: string; classification: string }
  | { type: 'System'; id: string };

export interface PrincipalExtractionHeaders {
  authorization?: string | undefined;
  'x-user-id'?: string | undefined;
  'x-org-id'?: string | undefined;
  'x-agent-id'?: string | undefined;
  'x-agent-classification'?: string | undefined;
}

const NAMESPACE = 'promptsheon';

/**
 * Build the Cedar UID shape the engine expects for a principal.
 * `__entity: { type, id }` is the canonical wire form.
 */
function uid(principal: Principal): cedar.EntityUidJson {
  switch (principal.type) {
    case 'User':
      return { __entity: { type: `${NAMESPACE}::User`, id: principal.id } };
    case 'Agent':
      return { __entity: { type: `${NAMESPACE}::Agent`, id: principal.id } };
    case 'System':
      return { __entity: { type: `${NAMESPACE}::System`, id: principal.id } };
  }
}

/**
 * Extract the principal from request headers. The lookup order:
 *
 *  1. `Authorization: Bearer <token>` → `User` principal.
 *     The userId is encoded in the JWT in production; for v1 the
 *     X-User-Id header is still consulted for legacy callers.
 *  2. `Authorization: SVID <token>` → `Agent` principal. SVID
 *     validation is the caller's responsibility (IN-0.4 only
 *     recognises the scheme; IN-0.5 will add a real verifier).
 *  3. `X-User-Id` + `X-Org-Id` → `User` principal. Legacy fallback
 *     for internal callers during the migration window.
 *  4. Nothing matches → `null` so the route layer can return
 *     401.
 *
 * The function is pure: it doesn't validate the token, doesn't
 * look anything up in the database, and doesn't fall open. The
 * route layer is responsible for the 401 on `null`.
 */
export function principalFromRequest(
  headers: PrincipalExtractionHeaders,
): Principal | null {
  const auth = headers.authorization;
  if (typeof auth === 'string' && auth.length > 0) {
    if (auth.startsWith('Bearer ')) {
      const userId = headers['x-user-id'];
      const orgId = headers['x-org-id'];
      if (typeof userId === 'string' && userId.length > 0) {
        if (typeof orgId === 'string' && orgId.length > 0) {
          return { type: 'User', id: userId, orgId, role: 'viewer' };
        }
        return { type: 'User', id: userId, orgId: 'unscoped', role: 'viewer' };
      }
      return null;
    }
    if (auth.startsWith('SVID ')) {
      const agentId = headers['x-agent-id'];
      if (typeof agentId !== 'string' || agentId.length === 0) return null;
      const classification = headers['x-agent-classification'] ?? 'internal';
      const orgId = headers['x-org-id'] ?? 'unscoped';
      return { type: 'Agent', id: agentId, orgId, classification };
    }
  }
  // Legacy: X-User-Id + X-Org-Id without Authorization.
  const userId = headers['x-user-id'];
  const orgId = headers['x-org-id'];
  if (typeof userId === 'string' && userId.length > 0) {
    if (typeof orgId === 'string' && orgId.length > 0) {
      return { type: 'User', id: userId, orgId, role: 'viewer' };
    }
    return null;
  }
  return null;
}

/**
 * Apply the configured system-actor override. When the env var
 * `PROMPTSHEON_ALLOW_SYSTEM_ACTOR=true` is set, an unauthenticated
 * caller becomes a `System` principal with full reach. Used by
 * bootstrap + replication; production deployments leave it off
 * by default.
 */
export function applySystemActorOverride(principal: Principal | null): Principal | null {
  if (principal !== null) return principal;
  if (process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] !== 'true') return null;
  return { type: 'System', id: 'bootstrap' };
}

/**
 * Convert the typed principal into the Cedar UID shape the engine
 * expects. Re-exported so the authorizer and the policy:eval CLI
 * don't have to repeat the namespace prefix logic.
 */
export function principalToUid(principal: Principal): cedar.EntityUidJson {
  return uid(principal);
}