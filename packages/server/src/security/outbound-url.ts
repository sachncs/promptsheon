import { isIP } from 'node:net';

export interface OutboundUrlPolicy {
  allowedHosts?: readonly string[];
  allowPrivateNetworks?: boolean;
}

export type OutboundUrlValidation =
  | { ok: true; url: URL }
  | { ok: false; reason: string };

/** Validate a user-supplied URL before the server makes an outbound request. */
export function validateOutboundUrl(raw: string, policy: OutboundUrlPolicy = {}): OutboundUrlValidation {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return { ok: false, reason: 'invalid URL' };
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return { ok: false, reason: 'only HTTP(S) URLs are supported' };
  }
  if (url.username || url.password) {
    return { ok: false, reason: 'URL credentials are not allowed' };
  }

  const hostname = url.hostname.toLowerCase().replace(/^\[|\]$/g, '');
  const allowedHosts = policy.allowedHosts?.map((host) => host.toLowerCase()).filter(Boolean) ?? [];
  if (allowedHosts.length > 0 && !allowedHosts.includes(hostname)) {
    return { ok: false, reason: 'host is not in the outbound allowlist' };
  }
  if (policy.allowPrivateNetworks !== true && isPrivateHostname(hostname)) {
    return { ok: false, reason: 'private and local network targets are not allowed' };
  }
  return { ok: true, url };
}

function isPrivateHostname(hostname: string): boolean {
  if (hostname === 'localhost' || hostname.endsWith('.localhost') || hostname.endsWith('.local') || hostname.endsWith('.internal')) {
    return true;
  }
  const version = isIP(hostname);
  if (version === 4) {
    const octets = hostname.split('.').map(Number);
    const [first, second] = octets;
    return first === 0 || first === 10 || first === 127 || first === 169 && second === 254 ||
      first === 172 && second >= 16 && second <= 31 || first === 192 && second === 168 ||
      first === 100 && second >= 64 && second <= 127;
  }
  if (version === 6) {
    return hostname === '::1' || hostname.startsWith('fc') || hostname.startsWith('fd') || hostname.startsWith('fe8') ||
      hostname.startsWith('fe9') || hostname.startsWith('fea') || hostname.startsWith('feb');
  }
  return hostname === 'metadata.google.internal';
}
