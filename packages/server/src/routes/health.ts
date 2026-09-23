import type { FastifyInstance } from 'fastify';
import type Database from 'better-sqlite3';

export function registerHealthRoutes(app: FastifyInstance, db: Database.Database) {
  app.get('/api/health', async (_request, reply) => {
    try {
      const result = db.prepare('SELECT 1 as ok').get() as { ok: number } | undefined;
      return reply.send({
        status: 'ok',
        db: result?.ok === 1 ? 'ok' : 'error',
        timestamp: new Date().toISOString(),
      });
    } catch (err) {
      app.log.error({ err }, 'health check database probe failed');
      return reply.code(503).send({
        status: 'error',
        db: 'error',
        error: 'database unavailable',
        timestamp: new Date().toISOString(),
      });
    }
  });

  app.get('/api/ready', async (_request, reply) => {
    try {
      const row = db.prepare('PRAGMA quick_check').get() as { quick_check?: string } | undefined;
      if (row?.quick_check !== 'ok') {
        return reply.code(503).send({
          status: 'not_ready',
          db: 'error',
          timestamp: new Date().toISOString(),
        });
      }
      return reply.send({ status: 'ready', db: 'ok', timestamp: new Date().toISOString() });
    } catch (err) {
      app.log.error({ err }, 'readiness database probe failed');
      return reply.code(503).send({
        status: 'not_ready',
        db: 'error',
        timestamp: new Date().toISOString(),
      });
    }
  });
}
