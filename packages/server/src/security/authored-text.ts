/** Flatten user-authored values while excluding object property names. */
export function authoredText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value.map(authoredText).join('\n');
  if (value && typeof value === 'object') {
    return Object.values(value as Record<string, unknown>).map(authoredText).join('\n');
  }
  return '';
}
