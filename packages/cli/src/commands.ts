import type { ApiClient } from './output.js';
import { BadArgsError, NotFoundError } from './errors.js';

export interface LoginResult {
  user: { id: string; email: string; role: string };
}

export async function loginCommand(client: ApiClient): Promise<LoginResult> {
  const me = await client.get<LoginResult | { error: unknown }>('/users/me');
  if ('error' in me) {
    throw new BadArgsError(`server returned error: ${JSON.stringify(me.error)}`);
  }
  return me as LoginResult;
}

export async function reposListCommand(client: ApiClient): Promise<Array<{ id: string; name: string; slug: string }>> {
  const ws = process.env['PROMPTSHEON_WORKSPACE_ID'];
  if (!ws) {
    throw new BadArgsError('PROMPTSHEON_WORKSPACE_ID required');
  }
  return client.get<Array<{ id: string; name: string; slug: string }>>(`/repos?workspaceId=${ws}`);
}

export interface EvalGateResult {
  ok: boolean;
  score: number;
}

export async function evalGateCommand(client: ApiClient, repoId: string): Promise<EvalGateResult> {
  if (!repoId) {
    throw new BadArgsError('eval gate <repoId> — repoId is required');
  }
  return client.post<EvalGateResult>(`/repos/${repoId}/eval-gate`, {
    trials: [{ caseId: 'sample', output: 'hello', finalState: {} }],
  });
}

export interface ReleaseSummary {
  id: string;
  status: string;
  capabilityId: string;
  version: number;
  manifestHash: string;
}

export async function releaseGetCommand(
  client: ApiClient,
  releaseId: string,
): Promise<ReleaseSummary> {
  if (!releaseId) throw new BadArgsError('release get <id> — id is required');
  const r = await client.get<ReleaseSummary | { error: unknown }>(`/releases/${releaseId}`);
  if ('error' in r) {
    throw new NotFoundError('release', releaseId);
  }
  return r as ReleaseSummary;
}

export interface ReleaseApproveResult {
  id: string;
  status: string;
  approvedBy: string[];
}

export async function releaseApproveCommand(
  client: ApiClient,
  releaseId: string,
  opts: { dryRun: boolean },
): Promise<ReleaseApproveResult | { dryRun: true; wouldPost: unknown }> {
  if (!releaseId) throw new BadArgsError('release approve <id> — id is required');
  if (opts.dryRun) {
    return {
      dryRun: true,
      wouldPost: {
        method: 'POST',
        path: `/releases/${releaseId}/approve`,
        body: { approverNote: '' },
      },
    };
  }
  return client.post<ReleaseApproveResult>(`/releases/${releaseId}/approve`, {
    approverNote: '',
  });
}

export interface ManifestScanResult {
  verdict: 'clean' | 'warn' | 'block';
  findingsCount: number;
}

export async function manifestScanCommand(
  client: ApiClient,
  manifestHash: string,
  opts: { dryRun: boolean },
): Promise<ManifestScanResult | { dryRun: true; wouldPost: unknown }> {
  if (!manifestHash) throw new BadArgsError('manifest scan <hash> — hash is required');
  if (opts.dryRun) {
    return {
      dryRun: true,
      wouldPost: {
        method: 'POST',
        path: `/manifests/${manifestHash}/scan`,
        body: {},
      },
    };
  }
  return client.post<ManifestScanResult>(`/manifests/${manifestHash}/scan`, {});
}