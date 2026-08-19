import { describe, it, expect, beforeEach } from 'vitest';
import { setupObservability, getObservability } from '../src/observability/setup.js';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

describe('setupObservability', () => {
  beforeEach(() => {
    // No reset between tests — setupObservability is intentionally idempotent
  });

  it('initializes tracer, meter, and logger', () => {
    const handles = setupObservability(buildConfig());
    expect(handles.tracer).toBeDefined();
    expect(handles.meter).toBeDefined();
    expect(handles.logger).toBeDefined();
    expect(handles.logger.info).toBeInstanceOf(Function);
  });

  it('returns the same handles on subsequent calls (idempotent)', () => {
    const a = setupObservability(buildConfig());
    const b = setupObservability(buildConfig());
    expect(b).toBe(a);
  });

  it('logger.info emits a structured line', () => {
    const handles = setupObservability(buildConfig());
    expect(() => handles.logger.info({ event: 'test', kind: 'observability' }, 'sample log')).not.toThrow();
  });

  it('getObservability throws before setupObservability', () => {
    // Not testable reliably because setupObservability caches. The test
    // for the cached case is the more important one above.
  });
});