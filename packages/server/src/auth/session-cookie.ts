/** Name of the HttpOnly cookie used by the browser control plane session. */
export const BROWSER_SESSION_COOKIE = 'promptsheon_session';

const COOKIE_MAX_AGE_SECONDS = 7 * 24 * 60 * 60;

/**
 * Read the browser session credential from a Cookie header.
 *
 * Cookie values are intentionally treated as opaque bearer credentials and
 * are never decoded or logged.
 */
export function readBrowserSessionCookie(header: string | undefined): string | undefined {
  if (!header) return undefined;
  for (const part of header.split(';')) {
    const separator = part.indexOf('=');
    if (separator < 0) continue;
    const name = part.slice(0, separator).trim();
    if (name !== BROWSER_SESSION_COOKIE) continue;
    const value = part.slice(separator + 1).trim();
    return value.length > 0 ? decodeCookieValue(value) : undefined;
  }
  return undefined;
}

/** Build a restrictive Set-Cookie value for the browser control plane. */
export function serializeBrowserSessionCookie(token: string, secure: boolean): string {
  const attributes = [
    `${BROWSER_SESSION_COOKIE}=${encodeURIComponent(token)}`,
    'Path=/',
    'HttpOnly',
    'SameSite=Lax',
    `Max-Age=${COOKIE_MAX_AGE_SECONDS}`,
  ];
  if (secure) attributes.push('Secure');
  return attributes.join('; ');
}

/** Build a Set-Cookie value that removes the browser control-plane session. */
export function serializeClearedBrowserSessionCookie(secure: boolean): string {
  const attributes = [
    `${BROWSER_SESSION_COOKIE}=`,
    'Path=/',
    'HttpOnly',
    'SameSite=Lax',
    'Max-Age=0',
  ];
  if (secure) attributes.push('Secure');
  return attributes.join('; ');
}

function decodeCookieValue(value: string): string | undefined {
  try {
    const decoded = decodeURIComponent(value);
    return decoded.length > 0 ? decoded : undefined;
  } catch {
    return undefined;
  }
}
