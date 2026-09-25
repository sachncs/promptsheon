/** Convert an arbitrary client-side failure into a safe user-facing message. */
export function getErrorMessage(error: unknown, fallback = 'We could not complete that request.'): string {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  if (typeof error === 'object' && error !== null && 'message' in error) {
    const message = error.message;
    if (typeof message === 'string' && message.trim().length > 0) return message;
  }
  if (typeof error === 'string' && error.trim().length > 0) return error;
  return fallback;
}
