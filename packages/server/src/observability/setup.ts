import { setupTracer, setupMeter, getTracer, getMeter } from '@strands-agents/sdk/telemetry';
import { configureLogging } from '@strands-agents/sdk';
import type { AppConfig } from '@promptsheon/shared';
import pino from 'pino';

/**
 * Initialize Strands observability stack:
 * - OTel tracing via setupTracer (if endpoint configured)
 * - OTel metrics via setupMeter
 * - Structured logging via configureLogging + pino
 *
 * Returns the OTel tracer + meter for use in custom instrumentation.
 * The setup is idempotent: re-running does not error.
 */
export interface ObservabilityHandles {
  tracer: ReturnType<typeof getTracer>;
  meter: ReturnType<typeof getMeter>;
  logger: pino.Logger;
}

let cached: ObservabilityHandles | null = null;

export function setupObservability(config: AppConfig): ObservabilityHandles {
  if (cached) return cached;

  const otelEndpoint = process.env['PROMPTSHEON_OTEL_ENDPOINT'];

  if (otelEndpoint) {
    try {
      setupTracer({ exporters: { otlp: true } });
    } catch (e) {
      console.error('[observability] setupTracer failed:', (e as Error).message);
    }
    try {
      setupMeter({ exporters: { otlp: true } });
    } catch (e) {
      console.error('[observability] setupMeter failed:', (e as Error).message);
    }
  }

  const logger = pino({
    level: config.server.logLevel,
    base: { service: 'promptsheon-server' },
  });

  try {
    configureLogging(logger);
  } catch (e) {
    // configureLogging may not accept pino directly; fall back to a console-backed logger
    console.error('[observability] configureLogging with pino failed, using console:', (e as Error).message);
    configureLogging(console as unknown as Parameters<typeof configureLogging>[0]);
  }

  cached = {
    tracer: getTracer(),
    meter: getMeter(),
    logger,
  };
  return cached;
}

export function getObservability(): ObservabilityHandles {
  if (!cached) {
    throw new Error('Observability not initialized. Call setupObservability() first.');
  }
  return cached;
}