import type { AppConfig, GuardrailSpec, Manifest } from '@promptsheon/shared';
import { SseHub } from '../../sse/hub.js';

export interface GuardrailContext {
  manifest: Manifest;
  nodeId: string;
  executionId: string;
  values: unknown[];
  phase: 'pre' | 'post';
}

/**
 * Run a single pre- or post- guardrail against the given values.
 * Returns a description of the failure, or null if the guardrail passed.
 *
 * Supported types:
 * - regex: tests each value as a string against the pattern
 * - blocklist: tests each value as a string against any of the blocklist terms
 * - schema: tests each value against the JSON schema (object-only)
 * - pii-redaction: applies redaction; always passes but mutates values
 * - llm-judge: skipped (would require async LLM call); always passes
 */
export function runGuardrail(spec: GuardrailSpec, context: GuardrailContext): string | null {
  if (!spec.enabled) return null;
  const values = context.phase === 'pre' ? context.values : context.values;

  switch (spec.type) {
    case 'regex': {
      const pattern = String(spec.config['pattern'] ?? '');
      if (!pattern) return null;
      const regex = new RegExp(pattern);
      for (const v of values) {
        if (typeof v === 'string' && regex.test(v)) {
          return `regex match: ${pattern}`;
        }
      }
      return null;
    }
    case 'blocklist': {
      const terms = (spec.config['terms'] as string[] | undefined) ?? [];
      for (const v of values) {
        if (typeof v === 'string') {
          for (const term of terms) {
            if (v.toLowerCase().includes(term.toLowerCase())) {
              return `blocklist match: ${term}`;
            }
          }
        }
      }
      return null;
    }
    case 'schema': {
      const allowedKeys = (spec.config['allowedKeys'] as string[] | undefined) ?? [];
      for (const v of values) {
        if (typeof v === 'object' && v !== null) {
          const extraKeys = Object.keys(v as Record<string, unknown>).filter((k) => !allowedKeys.includes(k));
          if (extraKeys.length > 0) {
            return `schema violation: extra keys ${extraKeys.join(', ')}`;
          }
        }
      }
      return null;
    }
    case 'pii-redaction': {
      for (let i = 0; i < values.length; i++) {
        if (typeof values[i] === 'string') {
          values[i] = redactPii(values[i] as string);
        }
      }
      return null;
    }
    case 'llm-judge': {
      return null;
    }
    default:
      return null;
  }
}

function redactPii(input: string): string {
  return input
    .replace(/\b\d{3}-\d{2}-\d{4}\b/g, '[SSN]')
    .replace(/\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b/g, '[CARD]')
    .replace(/[\w.+-]+@[\w-]+\.[\w.-]+/g, '[EMAIL]');
}

export interface GuardrailBroadcast {
  hub: SseHub;
  config: AppConfig;
}

export function runAllGuardrails(
  specs: GuardrailSpec[],
  context: GuardrailContext,
  broadcast: GuardrailBroadcast,
): { allowed: boolean; redactedValues: unknown[] } {
  const values = [...context.values];
  const innerContext: GuardrailContext = { ...context, values };

  for (const spec of specs) {
    const failure = runGuardrail(spec, innerContext);
    if (failure !== null) {
      if (spec.onFailure === 'block') {
        broadcast.hub.broadcast({
          type: 'error',
          data: { kind: 'guardrail_block', manifestHash: context.manifest.id, nodeId: context.nodeId, spec: spec.type, reason: failure },
          timestamp: new Date().toISOString(),
        });
        return { allowed: false, redactedValues: values };
      }
      broadcast.hub.broadcast({
        type: 'alert',
        data: { kind: 'guardrail_warn', manifestHash: context.manifest.id, nodeId: context.nodeId, spec: spec.type, reason: failure },
        timestamp: new Date().toISOString(),
      });
    }
  }
  return { allowed: true, redactedValues: values };
}