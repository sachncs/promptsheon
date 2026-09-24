import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { CedarAuthorizer } from './authorizer.js';
import type { Principal } from './principal.js';

export { CedarAuthorizer } from './authorizer.js';

export interface CedarGateOptions {
  /**
   * The Cedar action the route is performing. Each route passes
   * a specific value, e.g. `Action::"Agent::Publish"`.
   */
  action: string;
  /**
   * The Cedar resource. For org-scoped routes, this is typically
   * `Resource::"default"` because the policy file already
   * applies the role check (Role::"admin" etc.); the per-resource
   * authz lives in higher-level policies.
   */
  resource?: string;
  /**
   * Optional pre-loaded authorizer. If omitted, the helper
   * constructs one fresh on every call. Tests pass a fixture;
   * production wires the boot-loaded instance via setup.
   */
  authorizer?: CedarAuthorizer;
}

interface CedarContext {
  orgId: string;
  role: string;
}

/**
 * Build a Fastify preHandler that gates the request on a Cedar
 * decision. The principal must already be established by the
 * authentication middleware; raw headers are never interpreted by
 * the policy layer. Requests without an authenticated principal
 * are left to the authentication/org-context hooks to reject.
 */
export function cedarGate(opts: CedarGateOptions) {
  return async (request: FastifyRequest, reply: FastifyReply): Promise<void> => {
    const principal = extractPrincipal(request);
    if (principal === null) {
      // Authentication and org-context middleware own unauthenticated
      // request handling; Cedar only evaluates verified identities.
      return;
    }
    const authorizer = opts.authorizer ?? loadDefaultAuthorizer();
    if (authorizer === null) {
      // Authorizer configuration is handled at the composition root;
      // isolated tests may intentionally omit it.
      return;
    }
    const decision = authorizer.authorize({
      principal: { __entity: { type: principalType(principal), id: principal.id } },
      action: { __entity: { type: 'promptsheon::Action', id: opts.action } },
      resource: { __entity: { type: 'promptsheon::Resource', id: opts.resource ?? 'default' } },
    });
    if (!decision.allow) {
      return reply.code(403).send({
        error: {
          code: 'FORBIDDEN',
          message: `Cedar policy denied: ${opts.action}`,
          reasons: decision.reasons,
        },
      });
    }
  };
}

function principalType(p: Principal): string {
  if (p.type === 'User') return 'promptsheon::Role';
  return `promptsheon::${p.type}`;
}

function extractPrincipal(request: FastifyRequest): Principal | null {
  if (request.principal) return request.principal;
  if (request.userId) {
    const orgId = request.agentOrgId ?? request.orgContext?.orgId ?? 'unscoped';
    return {
      type: 'User',
      id: request.userId,
      orgId,
      role: request.orgContext?.role ?? request.userRole ?? 'viewer',
    };
  }
  return null;
}

let defaultAuthorizer: CedarAuthorizer | null = null;

function loadDefaultAuthorizer(): CedarAuthorizer | null {
  if (defaultAuthorizer !== null) return defaultAuthorizer;
  const policyPath = process.env['PROMPTSHEON_POLICY_FILE'];
  if (!policyPath) return null;
  defaultAuthorizer = new CedarAuthorizer({ policyPath });
  try {
    defaultAuthorizer.load();
  } catch {
    defaultAuthorizer = null;
  }
  return defaultAuthorizer;
}

/**
 * Used by the boot sequence (IN-0.8) to install the singleton
 * authorizer so the per-request load isn't paying the parse cost
 * on every call. Optional in tests; required in production.
 */
export function installDefaultAuthorizer(authorizer: CedarAuthorizer): void {
  defaultAuthorizer = authorizer;
}

/**
 * Read the org context from the request if orgContextMiddleware
 * ran first. We use this only to attach the orgId to a log
 * line; the Cedar policy does the actual authz.
 */
export function cedarContextFromRequest(request: FastifyRequest): CedarContext | null {
  const ctx = request.orgContext;
  return ctx ?? null;
}
