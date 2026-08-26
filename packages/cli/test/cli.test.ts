import { describe, it, expect } from 'vitest';
import { EXIT, PROMPTSHEON_CLI_VERSION } from '../src/version.js';

describe('CLI version + exit codes', () => {
  it('PROMPTSHEON_CLI_VERSION is a semver string', () => {
    expect(PROMPTSHEON_CLI_VERSION).toMatch(/^\d+\.\d+\.\d+$/);
  });
  it('exit codes are stable and locked', () => {
    expect(EXIT.OK).toBe(0);
    expect(EXIT.UNKNOWN).toBe(1);
    expect(EXIT.BAD_ARGS).toBe(2);
    expect(EXIT.API_ERROR).toBe(3);
    expect(EXIT.NETWORK_ERROR).toBe(4);
    expect(EXIT.AUTH_ERROR).toBe(5);
    expect(EXIT.NOT_FOUND).toBe(6);
    expect(EXIT.CONFLICT).toBe(7);
    expect(EXIT.PRECONDITION_FAILED).toBe(8);
  });
});

import { parseFlags, handleError, type ApiClient } from '../src/output.js';
import { BadArgsError, NetworkError, NotFoundError, AuthError } from '../src/errors.js';

describe('parseFlags', () => {
  it('returns text format by default', () => {
    const { positional, flags } = parseFlags(['login']);
    expect(positional).toEqual(['login']);
    expect(flags.format).toBe('text');
    expect(flags.dryRun).toBe(false);
  });
  it('parses --json', () => {
    const { flags } = parseFlags(['--json', 'login']);
    expect(flags.format).toBe('json');
  });
  it('parses --dry-run', () => {
    const { flags } = parseFlags(['--dry-run', 'release', 'approve', 'rel-1']);
    expect(flags.dryRun).toBe(true);
    expect(flags.format).toBe('text');
  });
  it('ignores flags interleaved with positional args', () => {
    const { positional, flags } = parseFlags(['release', '--json', 'approve', 'rel-1', '--dry-run']);
    expect(positional).toEqual(['release', 'approve', 'rel-1']);
    expect(flags.format).toBe('json');
    expect(flags.dryRun).toBe(true);
  });
});

describe('handleError exit-code mapping', () => {
  it('maps BadArgsError to BAD_ARGS', () => {
    expect(handleError(new BadArgsError('test'), 'text')).toBe(EXIT.BAD_ARGS);
  });
  it('maps NetworkError to NETWORK_ERROR', () => {
    expect(handleError(new NetworkError('boom'), 'text')).toBe(EXIT.NETWORK_ERROR);
  });
  it('maps NotFoundError to NOT_FOUND', () => {
    expect(handleError(new NotFoundError('release', 'rel-1'), 'text')).toBe(EXIT.NOT_FOUND);
  });
  it('maps AuthError to AUTH_ERROR', () => {
    expect(handleError(new AuthError(), 'text')).toBe(EXIT.AUTH_ERROR);
  });
  it('maps a generic Error to UNKNOWN', () => {
    expect(handleError(new Error('boom'), 'text')).toBe(EXIT.UNKNOWN);
  });
  it('emits JSON when format is json', () => {
    const out: string[] = [];
    const original = console.log;
    console.log = (...args: unknown[]) => out.push(args.join(' '));
    try {
      const code = handleError(new BadArgsError('x'), 'json');
      expect(code).toBe(EXIT.BAD_ARGS);
      expect(out.join(' ')).toContain('"code": 2');
      expect(out.join(' ')).toContain('"message": "x"');
    } finally {
      console.log = original;
    }
  });
});

import {
  loginCommand,
  reposListCommand,
  evalGateCommand,
  releaseGetCommand,
  releaseApproveCommand,
  manifestScanCommand,
} from '../src/commands.js';

function fakeClient(handler: (method: string, path: string, body: unknown) => unknown): ApiClient {
  return {
    get: async (path) => handler('GET', path, undefined) as never,
    post: async (path, body) => handler('POST', path, body) as never,
  };
}

describe('commands: argument validation', () => {
  it('evalGateCommand throws BadArgsError when repoId missing', async () => {
    await expect(evalGateCommand(fakeClient(() => null), '')).rejects.toThrow(/repoId/);
  });
  it('releaseGetCommand throws BadArgsError when id missing', async () => {
    await expect(releaseGetCommand(fakeClient(() => null), '')).rejects.toThrow(/id/);
  });
  it('releaseApproveCommand throws BadArgsError when id missing', async () => {
    await expect(
      releaseApproveCommand(fakeClient(() => null), '', { dryRun: false }),
    ).rejects.toThrow(/id/);
  });
  it('manifestScanCommand throws BadArgsError when hash missing', async () => {
    await expect(
      manifestScanCommand(fakeClient(() => null), '', { dryRun: false }),
    ).rejects.toThrow(/hash/);
  });
});

describe('commands: dry-run paths', () => {
  it('releaseApproveCommand dry-run does not call post', async () => {
    let called = false;
    const client = fakeClient(() => {
      called = true;
      return null;
    });
    const r = await releaseApproveCommand(client, 'rel-1', { dryRun: true });
    expect(called).toBe(false);
    expect((r as { dryRun: true }).dryRun).toBe(true);
  });
  it('manifestScanCommand dry-run does not call post', async () => {
    let called = false;
    const client = fakeClient(() => {
      called = true;
      return null;
    });
    const r = await manifestScanCommand(client, 'h1', { dryRun: true });
    expect(called).toBe(false);
    expect((r as { dryRun: true }).dryRun).toBe(true);
  });
});

describe('commands: real paths', () => {
  it('loginCommand returns the parsed user', async () => {
    const client = fakeClient(() => ({
      user: { id: 'u-1', email: 'a@b', role: 'admin' },
    }));
    const r = await loginCommand(client);
    expect(r.user.id).toBe('u-1');
  });
  it('reposListCommand requires PROMPTSHEON_WORKSPACE_ID', async () => {
    const prev = process.env['PROMPTSHEON_WORKSPACE_ID'];
    delete process.env['PROMPTSHEON_WORKSPACE_ID'];
    try {
      await expect(reposListCommand(fakeClient(() => []))).rejects.toThrow(/WORKSPACE_ID/);
    } finally {
      if (prev !== undefined) process.env['PROMPTSHEON_WORKSPACE_ID'] = prev;
    }
  });
  it('reposListCommand passes workspaceId through', async () => {
    process.env['PROMPTSHEON_WORKSPACE_ID'] = 'ws-x';
    const client = fakeClient((method, path) => {
      expect(method).toBe('GET');
      expect(path).toBe('/repos?workspaceId=ws-x');
      return [{ id: 'r1', name: 'repo', slug: 'r' }];
    });
    const r = await reposListCommand(client);
    expect(r).toHaveLength(1);
  });
  it('evalGateCommand returns ok + score', async () => {
    const client = fakeClient(() => ({ ok: true, score: 0.92 }));
    const r = await evalGateCommand(client, 'repo-1');
    expect(r.ok).toBe(true);
    expect(r.score).toBe(0.92);
  });
});