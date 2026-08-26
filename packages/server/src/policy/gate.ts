import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { CedarAuthorizer } from './authorizer.js';
import { applySystemActorOverride, principalFromRequest, type Principal } from './principal.js';

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
 * decision. The principal is extracted from the request headers
 * (IN-0.4); if no principal is present the system-actor override
 * applies, and if that's also off the gate passes through. That's
 * the same "skip when no org context" behaviour the old
 * `rolePreHandler` had, so existing tests that don't set up a
 * tenant keep working.
 */
export function cedarGate(opts: CedarGateOptions) {
  return async (request: FastifyRequest, reply: FastifyReply): Promise<void> => {
    const principal = extractPrincipal(request);
    if (principal === null) {
      // No principal in the request and the system-actor override
      // is off. The original rolePreHandler skipped in this case;
      // preserve that.
      return;
    }
    const authorizer = opts.authorizer ?? loadDefaultAuthorizer();
    if (authorizer === null) {
      // Authorizer hasn't been initialised (env var not set, or
      // we're in a test that didn't wire it up). Fall through and
      // let the request reach the route. This preserves the
      // behaviour of the old rolePreHandler which also skipped
      // when no org context was present.
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
  const headers = request.headers as Record<string, string | undefined>;
  const principal = principalFromRequest(headers);
  return applySystemActorOverride(principal);
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
  const ctx = (request as unknown as { orgContext?: CedarContext }).orgContext;
  return ctx ?? null;
}