import { describe, it, expect } from 'vitest';
import { createHash, getFips } from 'node:crypto';
import { computeHash } from '../src/audit/chain.js';

describe('FIPS gate (computeHash)', () => {
  it('produces the standard sha256 when fipsMode is false', () => {
    const h = computeHash('hello', false);
    expect(h).toMatch(/^[0-9a-f]{64}$/);
    expect(h).toBe(createHash('sha256').update('hello').digest('hex'));
    expect(h).toBe('2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824');
  });

  it('refuses to compute when fipsMode is true but the OpenSSL FIPS provider is not active', () => {
    // In a normal test environment Node is not running against a
    // FIPS-validated OpenSSL build, so getFips() returns 0 and the
    // gate must throw rather than silently downgrade.
    if (getFips() !== 1) {
      expect(() => computeHash('hello', true)).toThrowError(/FIPS/);
    }
  });

  it('falls through to the same sha256 when fipsMode is true and the provider is active', () => {
    // This branch only runs on a FIPS-validated Node build
    // (e.g. CI runner provisioned with OpenSSL FIPS module).
    if (getFips() === 1) {
      const h = computeHash('hello', true);
      expect(h).toBe(createHash('sha256').update('hello').digest('hex'));
    }
  });
});