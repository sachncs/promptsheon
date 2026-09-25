import { describe, expect, it, vi } from 'vitest';
import Fastify from 'fastify';
import { registerAnalyticsRoutes } from '../src/routes/analytics.js';

describe('user analytics route', () => {
  it('requires organization context and passes it to the repository', async () => {
    const perDay = vi.fn().mockReturnValue([]);
    const app = Fastify({ logger: false });
    registerAnalyticsRoutes(app, { repo: { perDay } } as never);
    await app.ready();

    const unauthorized = await app.inject({ method: 'GET', url: '/api/analytics/users/user-1?days=7' });
    expect(unauthorized.statusCode).toBe(401);

    await app.close();

    const scopedApp = Fastify({ logger: false });
    scopedApp.addHook('preHandler', (request, _reply, done) => {
      (request as Record<string, unknown>)['orgContext'] = { orgId: 'org-1' };
      done();
    });
    registerAnalyticsRoutes(scopedApp, { repo: { perDay } } as never);
    await scopedApp.ready();
    const authorized = await scopedApp.inject({ method: 'GET', url: '/api/analytics/users/user-1?days=7' });
    expect(authorized.statusCode).toBe(200);
    expect(perDay).toHaveBeenLastCalledWith('user-1', 'org-1', 7);
    await scopedApp.close();
  });
});
