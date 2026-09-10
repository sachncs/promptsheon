import type { AppConfig } from '@promptsheon/shared';

export function corsConfig(config: AppConfig) {
  return {
    origin: config.server.corsOrigin,
    methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'],
    allowedHeaders: ['Content-Type', 'Authorization', 'X-Request-Id', 'Idempotency-Key'],
    credentials: true,
  };
}
