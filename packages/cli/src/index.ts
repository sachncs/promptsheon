#!/usr/bin/env node
/**
 * Promptsheon CLI — minimal subcommand harness for talking to the
 * Fastify API. Designed for CI scripts and developer ergonomics;
 * not a re-implementation of the product UI.
 *
 * Auth via env:
 *   PROMPTSHEON_API_URL=http://127.0.0.1:8080
 *   PROMPTSHEON_API_KEY=<org-scoped bearer>
 */

interface ApiClient {
  get<T>(path: string): Promise<T>;
  post<T>(path: string, body: unknown): Promise<T>;
}

function makeClient(): ApiClient {
  const base = process.env['PROMPTSHEON_API_URL'] ?? 'http://127.0.0.1:8080';
  const key = process.env['PROMPTSHEON_API_KEY'] ?? '';

  async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const init: RequestInit = {
      method,
      headers: {
        'content-type': 'application/json',
        ...(key ? { authorization: `Bearer ${key}` } : {}),
      },
    };
    if (body !== undefined) init.body = JSON.stringify(body);
    const res = await fetch(`${base}/api${path}`, init);
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`${method.toUpperCase()} ${path} -> ${res.status}: ${text}`);
    }
    const text = await res.json().catch(() => ({}));
    return text as T;
  }
  return {
    get: (p) => call('GET', p),
    post: (p, b) => call('POST', p, b),
  };
}

function usage(): void {
  console.log(`promptsheon <command> [args]

Commands:
  login                 verify the API key works
  repos list            list repositories in the workspace
  eval gate <repoId>    run the CI eval gate for a repo
  release approve <id>  approve a release (maker-checker)
`);
}

async function main(argv: string[]): Promise<void> {
  const cmd = argv[2];
  if (!cmd || cmd === '--help' || cmd === '-h') {
    usage();
    return;
  }
  const client = makeClient();
  switch (cmd) {
    case 'login': {
      const me = await client.get<{ user: { id: string; email: string } } | { error: unknown }>('/users/me');
      console.log(JSON.stringify(me, null, 2));
      break;
    }
    case 'repos':
      if (argv[3] === 'list') {
        const ws = process.env['PROMPTSHEON_WORKSPACE_ID'];
        if (!ws) throw new Error('PROMPTSHEON_WORKSPACE_ID required');
        const list = await client.get<Array<{ id: string; name: string; slug: string }>>(`/repos?workspaceId=${ws}`);
        for (const r of list) console.log(`${r.id}  ${r.name}  (${r.slug})`);
        break;
      }
      usage();
      break;
    case 'eval':
      if (argv[3] === 'gate') {
        const repoId = argv[4];
        if (!repoId) throw new Error('eval gate <repoId>');
        const result = await client.post<{ ok: boolean; score: number }>(`/repos/${repoId}/eval-gate`, {
          trials: [{ caseId: 'sample', output: 'hello', finalState: {} }],
        });
        console.log(JSON.stringify(result));
        break;
      }
      usage();
      break;
    default:
      usage();
      process.exitCode = 1;
  }
}

main(process.argv).catch((err) => {
  console.error(String(err));
  process.exitCode = 1;
});
