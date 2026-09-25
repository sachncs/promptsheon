import { describe, expect, it } from 'vitest';
import { validateOutboundUrl } from '../src/security/outbound-url.js';

describe('validateOutboundUrl', () => {
  it.each([
    'http://localhost:8080/health',
    'http://127.0.0.1:8080/health',
    'http://169.254.169.254/latest/meta-data',
    'http://10.0.0.5/internal',
    'http://service.internal/api',
  ])('rejects private target %s', (url) => {
    expect(validateOutboundUrl(url)).toMatchObject({ ok: false });
  });

  it('requires HTTP(S) and rejects credentials', () => {
    expect(validateOutboundUrl('file:///etc/passwd')).toMatchObject({ ok: false });
    expect(validateOutboundUrl('https://user:pass@example.com')).toMatchObject({ ok: false });
  });

  it('enforces an explicit host allowlist', () => {
    expect(validateOutboundUrl('https://api.example.com/run', { allowedHosts: ['api.example.com'] })).toMatchObject({ ok: true });
    expect(validateOutboundUrl('https://other.example.com/run', { allowedHosts: ['api.example.com'] })).toMatchObject({ ok: false });
  });
});
