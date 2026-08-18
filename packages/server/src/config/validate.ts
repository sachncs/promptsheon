import type { AppConfig } from '@promptsheon/shared';

export function validateConfig(config: AppConfig): void {
  if (config.auth.enabled && !config.auth.jwtSecret) {
    throw new Error('PROMPTSHEON_JWT_SECRET is required when auth is enabled');
  }
  if (config.server.port < 1 || config.server.port > 65535) {
    throw new Error('PROMPTSHEON_PORT must be between 1 and 65535');
  }
}
