import { createHash } from 'node:crypto';

const SENSITIVE_KEY = /(?:api[-_]?key|authorization|cookie|password|secret|token|credential|private[-_]?key|access[-_]?key)/i;
const PII_PATTERNS: ReadonlyArray<readonly [RegExp, string]> = [
  [/\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/gi, '[REDACTED_EMAIL]'],
  [/\b\d{3}-\d{2}-\d{4}\b/g, '[REDACTED_SSN]'],
  [/\b(?:\d[ -]*?){13,19}\b/g, '[REDACTED_CARD]'],
];

export type RedactionSensitivity = 'public' | 'internal' | 'sensitive';

/** Recursively removes credentials and common PII before telemetry leaves a process. */
export function redactTelemetry(value: unknown, sensitivity: RedactionSensitivity = 'internal'): unknown {
  if (sensitivity === 'public') return value;
  if (typeof value === 'string') {
    return PII_PATTERNS.reduce((text, [pattern, replacement]) => text.replace(pattern, replacement), value);
  }
  if (Array.isArray(value)) return value.map((item) => redactTelemetry(item, sensitivity));
  if (value && typeof value === 'object') {
    const output: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      output[key] = SENSITIVE_KEY.test(key) ? '[REDACTED]' : redactTelemetry(item, sensitivity);
    }
    return output;
  }
  return value;
}

/** Stable JSON encoding used for evidence hashes and export verification. */
export function canonicalTelemetryJson(value: unknown): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalTelemetryJson).join(',')}]`;
  const record = value as Record<string, unknown>;
  return `{${Object.keys(record).sort().map((key) => `${JSON.stringify(key)}:${canonicalTelemetryJson(record[key])}`).join(',')}}`;
}

/** Hashes the redacted canonical payload stored with an evidence record. */
export function hashTelemetry(value: unknown): string {
  return createHash('sha256').update(canonicalTelemetryJson(value), 'utf8').digest('hex');
}
