import type { AppConfig } from '@promptsheon/shared';

export function rateLimitConfig(_config: AppConfig) {
  return {
    max: 100,
    timeWindow: '1 minute',
    keyGenerator: (request: Record<string, unknown>) => {
      return (request.userId as string) ?? (request.ip as string) ?? 'unknown';
    },
    errorResponseBuilder: () => ({
      error: {
        code: 'RATE_LIMITED',
        message: 'Too many requests',
      },
    }),
  };
}
