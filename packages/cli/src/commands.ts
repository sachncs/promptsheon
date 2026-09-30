import type { ApiClient } from './output.js';
import { BadArgsError, NotFoundError } from './errors.js';
import { readFile } from 'node:fs/promises';

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

function workspaceId(): string {
  const id = process.env['PROMPTSHEON_WORKSPACE_ID'];
  if (!id) throw new BadArgsError('PROMPTSHEON_WORKSPACE_ID required');
  return id;
}

async function specificationFile(path: string): Promise<unknown> {
  if (!path) throw new BadArgsError('spec file path is required');
  try {
    return JSON.parse(await readFile(path, 'utf8')) as unknown;
  } catch (error) {
    throw new BadArgsError(`cannot read specification file: ${error instanceof Error ? error.message : String(error)}`);
  }
}

export async function specificationValidateCommand(client: ApiClient, path: string): Promise<unknown> {
  return client.post(`/workspaces/${workspaceId()}/agent-specifications/validate`, { specification: await specificationFile(path) });
}

export async function specificationListCommand(client: ApiClient): Promise<unknown> {
  const params = new URLSearchParams();
  const page = optionalPositiveInteger('PROMPTSHEON_PAGE');
  const pageSize = optionalPositiveInteger('PROMPTSHEON_PAGE_SIZE');
  const status = process.env['PROMPTSHEON_SPEC_STATUS'];
  if (page !== undefined) params.set('page', String(page));
  if (pageSize !== undefined) params.set('pageSize', String(pageSize));
  if (status !== undefined) {
    if (!['draft', 'candidate', 'published', 'retired'].includes(status)) {
      throw new BadArgsError('PROMPTSHEON_SPEC_STATUS must be draft, candidate, published, or retired');
    }
    params.set('status', status);
  }
  const query = params.toString();
  return client.get(`/workspaces/${workspaceId()}/agent-specifications${query ? `?${query}` : ''}`);
}

export async function specificationCreateCommand(client: ApiClient, path: string, opts: { dryRun: boolean }): Promise<unknown> {
  const parentHash = process.env['PROMPTSHEON_PARENT_HASH'];
  if (parentHash !== undefined && !/^[0-9a-f]{64}$/.test(parentHash)) {
    throw new BadArgsError('PROMPTSHEON_PARENT_HASH must be a 64-character lowercase SHA-256 hash');
  }
  return client.post(`/workspaces/${workspaceId()}/agent-specifications`, {
    specification: await specificationFile(path),
    changeReason: process.env['PROMPTSHEON_CHANGE_REASON'] ?? 'created from CLI',
    ...(parentHash === undefined ? {} : { parentHash }),
  }, opts);
}

export async function specificationGetCommand(client: ApiClient, hash: string): Promise<unknown> {
  if (!hash) throw new BadArgsError('spec get <hash> — hash is required');
  return client.get(`/workspaces/${workspaceId()}/agent-specifications/${hash}`);
}

export async function specificationLineageCommand(client: ApiClient, hash: string): Promise<unknown> {
  if (!hash) throw new BadArgsError('spec lineage <hash> — hash is required');
  return client.get(`/workspaces/${workspaceId()}/agent-specifications/${hash}/lineage`);
}

export async function specificationDiffCommand(client: ApiClient, leftHash: string, rightHash: string): Promise<unknown> {
  if (!leftHash || !rightHash) throw new BadArgsError('spec diff <leftHash> <rightHash> — both hashes are required');
  return client.post(`/workspaces/${workspaceId()}/agent-specifications/diff`, { leftHash, rightHash });
}

export async function specificationPublishCommand(client: ApiClient, hash: string, opts: { dryRun: boolean }): Promise<unknown> {
  if (!hash) throw new BadArgsError('spec publish <hash> — hash is required');
  return client.post(`/workspaces/${workspaceId()}/agent-specifications/${hash}/publish`, {}, opts);
}

export async function executionSubmitCommand(client: ApiClient, agentHash: string, inputPath: string, idempotencyKey: string, opts: { dryRun: boolean }): Promise<unknown> {
  if (!agentHash || !/^[0-9a-f]{64}$/.test(agentHash)) throw new BadArgsError('execution submit <agentHash> <input-file> <idempotency-key> — a 64-character agent hash is required');
  if (!idempotencyKey) throw new BadArgsError('execution submit — idempotency key is required');
  return client.post(`/workspaces/${workspaceId()}/execution-jobs`, { agentHash, inputs: await specificationFile(inputPath), idempotencyKey }, opts);
}

export async function executionGetCommand(client: ApiClient, id: string): Promise<unknown> {
  if (!id) throw new BadArgsError('execution get <id> — id is required');
  return client.get(`/workspaces/${workspaceId()}/execution-jobs/${id}`);
}

export async function executionCancelCommand(client: ApiClient, id: string, opts: { dryRun: boolean }): Promise<unknown> {
  if (!id) throw new BadArgsError('execution cancel <id> — id is required');
  return client.post(`/workspaces/${workspaceId()}/execution-jobs/${id}/cancel`, {}, opts);
}

export async function executionMetricsCommand(client: ApiClient): Promise<unknown> {
  return client.get('/execution-jobs/metrics');
}

function optionalPositiveInteger(name: string): number | undefined {
  const raw = process.env[name];
  if (raw === undefined) return undefined;
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < 1) throw new BadArgsError(`${name} must be a positive integer`);
  return value;
}

export async function evidenceListCommand(client: ApiClient, options: { agentHash?: string; eventType?: string; traceId?: string } = {}): Promise<unknown> {
  const params = new URLSearchParams();
  if (options.agentHash) params.set('agentHash', options.agentHash);
  if (options.eventType) params.set('eventType', options.eventType);
  if (options.traceId) params.set('traceId', options.traceId);
  const query = params.toString();
  return client.get(`/evidence${query ? `?${query}` : ''}`);
}

export async function evidenceTraceCommand(client: ApiClient, traceId: string): Promise<unknown> {
  if (!traceId) throw new BadArgsError('evidence trace <traceId> — traceId is required');
  return client.get(`/traces/${encodeURIComponent(traceId)}/evidence`);
}
