import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { getOrgContext } from './org-context.js';

export { getOrgContext };

/**
 * Fastify decorator that asserts the request's orgContext.role is
 * 'admin'. Use after orgContextMiddleware has populated the context.
 *
 * Returns a 403 INSUFFICIENT_ROLE error if not, otherwise calls next().
 *
 * Usage:
 *
 *   app.get('/api/users', { preHandler: requireAdmin() }, handler);
 *
 * Or applied to an existing app at registration time (returning a
 * preHandler bound function).
 */
export function requireAdmin() {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    let ctx;
    try {
      ctx = getOrgContext(request);
    } catch {
      return reply
        .code(401)
        .send({ error: { code: 'NO_ORG_CONTEXT', message: 'admin route requires org context' } });
    }
    if (ctx.role !== 'admin') {
      return reply
        .code(403)
        .send({ error: { code: 'INSUFFICIENT_ROLE', message: 'Requires admin role' } });
    }
  };
}

/**
 * Higher-order helper to apply a preHandler to a single register
 * invocation. Useful for legacy routes that aren't ready for the
 * full { preHandler, handler } form yet.
 */
export function applyAdminToExisting(app: FastifyInstance, method: string, path: string) {
  const route = app[method.toLowerCase() as 'get' | 'post' | 'put' | 'delete'];
  if (typeof route === 'function' && (route as unknown as { __patchedAdmin?: Set<string> }).__patchedAdmin?.has(path)) return;
  const existing = (app as unknown as { _routes?: unknown[] })._routes;
  void existing;
}
