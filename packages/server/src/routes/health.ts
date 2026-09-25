import type { FastifyInstance } from 'fastify';
import type { HealthService } from '../application/health-service.js';

export function registerHealthRoutes(app: FastifyInstance, service: HealthService) {
  app.get('/api/health', async (_request, reply) => {
    try {
      const healthy = service.isHealthy();
      const timestamp = new Date().toISOString();
      if (!healthy) {
        return reply.code(503).send({
          status: 'error',
          db: 'error',
          error: 'database unavailable',
          timestamp,
        });
      }
      return reply.send({ status: 'ok', db: 'ok', timestamp });
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
      if (!service.isReady()) {
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
