import type {
  Repository,
  RepositoryCreateInput,
  EvalSuite,
  EvalSuiteVersion,
  EvalSuiteRunInput,
  SigningKey,
} from '@promptsheon/shared';

/**
 * Typed fetch wrapper over the public REST API.
 *
 * Auth is via org-scoped API key (Bearer). The server is configured
 * to issue keys through /api/api-keys (Phase 3 follow-up); for
 * v0.1 the SDK uses the existing X-User-Id / X-Org-Id internal
 * auth path until a public Bearer route is added.
 */
export interface SdkOptions {
  baseUrl?: string;
  apiKey?: string;
  orgId?: string;
  userId?: string;
}

interface RequestOptions {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path: string;
  body?: unknown;
}

export class PromptsheonClient {
  private readonly baseUrl: string;
  private readonly apiKey: string | undefined;
  private readonly orgId: string | undefined;
  private readonly userId: string | undefined;

  constructor(opts: SdkOptions = {}) {
    this.baseUrl = opts.baseUrl ?? 'http://127.0.0.1:8080';
    this.apiKey = opts.apiKey;
    this.orgId = opts.orgId;
    this.userId = opts.userId;
  }

  private async call<T>({ method, path, body }: RequestOptions): Promise<T> {
    const headers: Record<string, string> = { 'content-type': 'application/json' };
    if (this.apiKey) headers['authorization'] = `Bearer ${this.apiKey}`;
    if (this.orgId) headers['x-org-id'] = this.orgId;
    if (this.userId) headers['x-user-id'] = this.userId;
    const init: RequestInit = { method, headers };
    if (body !== undefined) init.body = JSON.stringify(body);
    const res = await fetch(`${this.baseUrl}/api${path}`, init);
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`${method} ${path} -> ${res.status}: ${text}`);
    }
    return (await res.json()) as T;
  }

  // ---- Repositories
  listRepos(workspaceId: string): Promise<Repository[]> {
    return this.call({ method: 'GET', path: `/repos?workspaceId=${workspaceId}` });
  }

  createRepo(input: RepositoryCreateInput): Promise<Repository> {
    return this.call({ method: 'POST', path: '/repos', body: input });
  }

  // ---- Branches / contents / commits
  listBranches(repoId: string): Promise<unknown[]> {
    return this.call({ method: 'GET', path: `/repos/${repoId}/branches` });
  }

  putFile(repoId: string, path: string, content: string, ref = 'main'): Promise<unknown> {
    return this.call({
      method: 'PUT',
      path: `/repos/${repoId}/contents/${path}?ref=${ref}`,
      body: { path, content, ref },
    });
  }

  commit(repoId: string, ref: string, message: string, parents?: string[]): Promise<unknown> {
    return this.call({
      method: 'POST',
      path: `/repos/${repoId}/commits`,
      body: { ref, message, parents },
    });
  }

  // ---- Merge requests
  openMergeRequest(input: {
    repositoryId: string;
    title: string;
    description?: string;
    sourceBranch: string;
    targetBranch: string;
    sourceCommitOid: string;
  }): Promise<unknown> {
    return this.call({ method: 'POST', path: `/repos/${input.repositoryId}/merge-requests`, body: input });
  }

  decideMergeRequest(id: string, decision: 'approve' | 'request_changes', comment?: string): Promise<unknown> {
    return this.call({ method: 'POST', path: `/merge-requests/${id}/decisions`, body: { decision, comment } });
  }

  // ---- Signing
  uploadSigningKey(organizationId: string, label: string, publicKeyPem: string): Promise<SigningKey> {
    return this.call({
      method: 'POST',
      path: `/orgs/${organizationId}/signing-keys`,
      body: { organizationId, label, publicKeyPem },
    });
  }

  signCommit(oid: string, keyId: string, signature: string): Promise<unknown> {
    return this.call({
      method: 'POST',
      path: `/commits/${oid}/sign`,
      body: { keyId, signature },
    });
  }

  verifyCommit(oid: string): Promise<{ valid: boolean; reason?: string | null }> {
    return this.call({ method: 'GET', path: `/commits/${oid}/verify` });
  }

  // ---- Eval suites
  listSuites(capabilityId?: string): Promise<EvalSuite[]> {
    const q = capabilityId ? `?capabilityId=${capabilityId}` : '';
    return this.call({ method: 'GET', path: `/eval-suites${q}` });
  }

  createSuite(input: {
    capabilityId: string;
    name: string;
    description?: string;
    passThreshold?: number;
    borderlineBand?: number;
    initialGraders?: Array<{ name: string; kind: string; weight: number; config: unknown }>;
  }): Promise<{ suite: EvalSuite; version: EvalSuiteVersion }> {
    return this.call({ method: 'POST', path: '/eval-suites', body: input });
  }

  runSuite(suiteId: string, input: Partial<EvalSuiteRunInput> & {
    trials?: Array<{
      caseId: string; output: string; transcript?: string; finalState?: Record<string, unknown>;
      toolCalls?: Array<{ tool: string; args: Record<string, unknown>; result?: unknown }>;
      referenceTranscript?: string;
    }>;
  }): Promise<unknown> {
    return this.call({ method: 'POST', path: `/eval-suites/${suiteId}/run`, body: input ?? {} });
  }

  evalGate(repoId: string, trials: Array<{ caseId: string; output: string; finalState?: Record<string, unknown> }>): Promise<unknown> {
    return this.call({ method: 'POST', path: `/repos/${repoId}/eval-gate`, body: { trials } });
  }
}
