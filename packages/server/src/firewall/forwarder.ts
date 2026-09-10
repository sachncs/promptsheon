import type { Finding } from '../repos/prompt-scan.js';

export interface ForwardRequest {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: string;
}

export interface ForwardResponse {
  status: number;
  headers: Record<string, string>;
  body: string;
}

/**
 * HTTP forwarder used by the firewall sidecar. POSTs the request
 * body to the upstream LLM endpoint and returns the response.
 *
 * Errors here are mapped to a 502 Bad Gateway with the upstream
 * error in the body so the sidecar stays transparent to callers.
 */
export async function forwardToUpstream(req: ForwardRequest): Promise<ForwardResponse> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 120_000);
  try {
    const res = await fetch(req.url, {
      method: req.method,
      headers: req.headers,
      body: req.body,
      signal: controller.signal,
    });
    const responseHeaders: Record<string, string> = {};
    res.headers.forEach((value, key) => {
      responseHeaders[key] = value;
    });
    const body = await res.text();
    return { status: res.status, headers: responseHeaders, body };
  } catch (err) {
    const reason = (err as Error).message ?? 'upstream error';
    return {
      status: 502,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ error: { code: 'UPSTREAM_UNREACHABLE', message: reason } }),
    };
  } finally {
    clearTimeout(timeout);
  }
}

export function warningsToHeader(findings: Finding[]): string {
  if (findings.length === 0) return 'none';
  const summary = findings
    .slice(0, 5)
    .map((f) => `${f.severity}:${f.rule}`)
    .join(',');
  return findings.length > 5 ? `${summary} (+${findings.length - 5} more)` : summary;
}