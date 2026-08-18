const BLOCKED_HOSTS = ['localhost', '127.0.0.1', '0.0.0.0', '::1', '169.254.169.254'];
const BLOCKED_SCHEMES = ['file:', 'ftp:', 'gopher:'];

export function validateUrl(urlString: string): { valid: boolean; error?: string } {
  try {
    const url = new URL(urlString);

    if (BLOCKED_SCHEMES.includes(url.protocol)) {
      return { valid: false, error: `Blocked scheme: ${url.protocol}` };
    }

    if (BLOCKED_HOSTS.includes(url.hostname)) {
      return { valid: false, error: `Blocked host: ${url.hostname}` };
    }

    if (/^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/.test(url.hostname)) {
      return { valid: false, error: `Blocked private IP: ${url.hostname}` };
    }

    return { valid: true };
  } catch {
    return { valid: false, error: 'Invalid URL' };
  }
}
