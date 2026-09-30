import { redactTelemetry } from './redaction.js';

const MAX_ERROR_MESSAGE_LENGTH = 2_000;

/** Converts an unknown failure into a bounded, telemetry-safe message. */
export function safeErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error);
  const redacted = redactTelemetry(raw);
  const message = typeof redacted === 'string' ? redacted : String(redacted);
  return message.length > MAX_ERROR_MESSAGE_LENGTH
    ? `${message.slice(0, MAX_ERROR_MESSAGE_LENGTH)}…`
    : message;
}
