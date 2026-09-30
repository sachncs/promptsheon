import { describe, expect, it } from 'vitest';
import {
  readBrowserSessionCookie,
  serializeBrowserSessionCookie,
  serializeClearedBrowserSessionCookie,
} from '../src/auth/session-cookie.js';

describe('browser session cookie', () => {
  it('round-trips encoded credentials without exposing cookie attributes to the value', () => {
    const cookie = serializeBrowserSessionCookie('pk_token;with spaces', true);
    expect(cookie).toContain('HttpOnly');
    expect(cookie).toContain('Secure');
    expect(readBrowserSessionCookie(cookie)).toBe('pk_token;with spaces');
  });

  it('ignores malformed encoded values', () => {
    expect(readBrowserSessionCookie('promptsheon_session=%E0%A4%A')).toBeUndefined();
    expect(readBrowserSessionCookie('other=value')).toBeUndefined();
  });

  it('clears the cookie with an expired max age', () => {
    const cookie = serializeClearedBrowserSessionCookie(false);
    expect(cookie).toContain('Max-Age=0');
    expect(cookie).not.toContain('Secure');
  });
});
