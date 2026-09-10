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
      return reply.code(503).send({
        status: 'error',
        db: 'error',
        error: String(err),
        timestamp: new Date().toISOString(),
      });
    }
  });
}
