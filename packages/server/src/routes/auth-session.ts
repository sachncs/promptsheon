import type { FastifyInstance } from 'fastify';
import {
  serializeBrowserSessionCookie,
  serializeClearedBrowserSessionCookie,
} from '../auth/session-cookie.js';

/** Register the browser control-plane session cookie exchange endpoints. */
export function registerAuthSessionRoutes(app: FastifyInstance, nodeEnvironment: string): void {
  const secure = nodeEnvironment === 'production';

  app.post('/api/auth/session', async (request, reply) => {
    const authorization = request.headers.authorization;
    if (typeof authorization !== 'string' || !authorization.startsWith('Bearer ') || authorization.length <= 7) {
      return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'A Bearer credential is required' } });
    }
    reply.header('Set-Cookie', serializeBrowserSessionCookie(authorization.slice(7), secure));
    return reply.send({ authenticated: true });
  });

  app.delete('/api/auth/logout', async (_request, reply) => {
    reply.header('Set-Cookie', serializeClearedBrowserSessionCookie(secure));
    return reply.send({ authenticated: false });
  });
}
