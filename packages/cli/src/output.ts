import { CliError, NetworkError, ApiError, AuthError } from './errors.js';

export interface OutputMode {
  /** 'text' prints human-friendly lines; 'json' prints the raw payload. */
  format: 'text' | 'json';
  /** When set, mutating commands print the would-be request and exit 0 without firing it. */
  dryRun: boolean;
}

export function parseFlags(argv: string[]): { positional: string[]; flags: OutputMode } {
  const positional: string[] = [];
  let format: 'text' | 'json' = 'text';
  let dryRun = false;
  for (const a of argv) {
    if (a === '--json') format = 'json';
    else if (a === '--dry-run') dryRun = true;
    else if (!a.startsWith('--')) positional.push(a);
  }
  return { positional, flags: { format, dryRun } };
}

export interface ApiClient {
  get<T>(path: string): Promise<T>;
  post<T>(path: string, body: unknown, opts?: { dryRun?: boolean }): Promise<T>;
}

export function makeClient(): ApiClient {
  const base = process.env['PROMPTSHEON_API_URL'] ?? 'http://127.0.0.1:8080';
  const key = process.env['PROMPTSHEON_API_KEY'] ?? '';

  async function call<T>(
    method: string,
    path: string,
    body: unknown,
    dryRun: boolean,
  ): Promise<T> {
    const init: RequestInit = {
      method,
      headers: {
        'content-type': 'application/json',
        ...(key ? { authorization: `Bearer ${key}` } : {}),
      },
    };
    if (body !== undefined) init.body = JSON.stringify(body);
    if (dryRun) {
      // Surface the would-be request and return early so the caller
      // can format it for humans or pipes.
      const out = {
        dryRun: true,
        method,
        url: `${base}/api${path}`,
        body: body ?? null,
        headers: init.headers as Record<string, string>,
      };
      return out as unknown as T;
    }
    let res: Response;
    try {
      res = await fetch(`${base}/api${path}`, init);
    } catch (err) {
      const reason = (err as Error).message ?? 'network error';
      throw new NetworkError(`${method.toUpperCase()} ${path}: ${reason}`);
    }
    const text = await res.text().catch(() => '');
    let parsed: unknown = {};
    try {
      parsed = text ? JSON.parse(text) : {};
    } catch {
      parsed = text;
    }
    if (res.status === 401 || res.status === 403) {
      throw new AuthError(`${res.status}: ${text}`);
    }
    if (res.status === 404) {
      throw new ApiError(404, `not found`, parsed);
    }
    if (!res.ok) {
      throw new ApiError(res.status, text || res.statusText, parsed);
    }
    return parsed as T;
  }
  return {
    get: (p) => call('GET', p, undefined, false),
    post: (p, b, opts) => call('POST', p, b, opts?.dryRun ?? false),
  };
}

export function print(format: 'text' | 'json', payload: unknown): void {
  if (format === 'json') {
    console.log(JSON.stringify(payload, null, 2));
    return;
  }
  if (payload === undefined || payload === null) {
    console.log('(no output)');
    return;
  }
  if (typeof payload === 'string') {
    console.log(payload);
    return;
  }
  if (Array.isArray(payload)) {
    for (const item of payload) console.log(formatLine(item));
    return;
  }
  console.log(formatLine(payload));
}

function formatLine(value: unknown): string {
  if (value && typeof value === 'object') {
    const obj = value as Record<string, unknown>;
    const id = obj['id'] ?? obj['slug'] ?? '';
    const name = obj['name'] ?? obj['label'] ?? obj['title'] ?? '';
    return `${id}  ${name}`.trim();
  }
  return String(value);
}

export function handleError(err: unknown, format: 'text' | 'json'): number {
  if (err instanceof CliError) {
    if (format === 'json') {
      console.log(JSON.stringify({ error: { code: err.code, message: err.message, detail: err.detail ?? null } }, null, 2));
    } else {
      console.error(`error: ${err.message}`);
      if (err.detail && format === 'text') {
        console.error(JSON.stringify(err.detail, null, 2));
      }
    }
    return err.code;
  }
  const msg = err instanceof Error ? err.message : String(err);
  if (format === 'json') {
    console.log(JSON.stringify({ error: { code: 'UNKNOWN', message: msg } }, null, 2));
  } else {
    console.error(`error: ${msg}`);
  }
  return 1;
}