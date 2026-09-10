const SECRET_SEGMENTS = ['key', 'secret', 'password', 'token', 'credential'];

export function isSecretKey(key: string): boolean {
  const lower = key.toLowerCase();
  return SECRET_SEGMENTS.some((s) => lower.includes(s));
}

export function maskSecret(value: string): string {
  if (value.length <= 8) return '****';
  return value.slice(0, 4) + '****' + value.slice(-4);
}
