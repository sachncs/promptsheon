/** Convert an arbitrary client-side failure into a safe user-facing message. */
export function getErrorMessage(error: unknown, fallback = 'We could not complete that request.'): string {
  let message: string | undefined;
  if (error instanceof Error && error.message.trim().length > 0) message = error.message;
  if (typeof error === 'object' && error !== null && 'message' in error) {
    const candidate = error.message;
    if (typeof candidate === 'string' && candidate.trim().length > 0) message = candidate;
  }
  if (!message && typeof error === 'string' && error.trim().length > 0) message = error;
  if (!message) return fallback;
  return sanitizeErrorMessage(message);
}

const SENSITIVE_VALUE = /(["']?(?:api[-_]?key|authorization|password|secret|token|credential|private[-_]?key|access[-_]?key)["']?\s*[:=]\s*["']?)[^"',;\s}]+/gi;
const MAX_ERROR_LENGTH = 500;

function sanitizeErrorMessage(message: string): string {
  const redacted = message.replace(SENSITIVE_VALUE, '$1[REDACTED]');
  return redacted.length > MAX_ERROR_LENGTH ? `${redacted.slice(0, MAX_ERROR_LENGTH)}…` : redacted;
}
