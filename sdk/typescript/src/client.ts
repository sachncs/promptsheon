/**
 * TypeScript client for the Promptsheon v1 API.
 *
 * Today this file is hand-written against the public resource list
 * in the architecture review (§7). The M3 follow-on commit runs
 * `npx openapi-typescript api/openapi.yaml` to regenerate this from
 * the produced spec; today the package compiles against a stub
 * `paths` type so consumers can adopt the SDK without waiting on
 * the codegen pipeline.
 *
 * SDK-3: every /api/v1 route is wrapped here. `make sdk`
 * regenerates the OpenAPI stub at sdk/typescript/src/_generated/.
 */
export interface Capability {
  id: string;
  project_id: string;
  name: string;
  description?: string;
  owner?: string;
  tags?: string[];
  contract?: CapabilityContract;
  created_at: string;
  updated_at: string;
}

export interface CapabilityContract {
  blast_radius: "low" | "medium" | "high";
  success_rubric?: string;
  auto_promotable?: boolean;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  slo_target?: SLOTarget;
}

export interface SLOTarget {
  max_p95_latency_ms?: number;
  min_success_rate?: number;
  max_hallucination_rate?: number;
}

export interface Reputation {
  capability_id: string;
  trust_score: number;
  eval_pass_rate: number;
  slo_adherence_rate: number;
  decision_adoption_rate: number;
  sample_size: number;
}

export interface ManifestDiff {
  from_version: number;
  to_version: number;
  added: ArtifactRef[];
  removed: ArtifactRef[];
  changed: { kind: string; old_hash: string; new_hash: string }[];
}

export interface ArtifactRef {
  kind: string;
  hash: string;
}

export interface Release {
  id: string;
  capability_id: string;
  capability_version: number;
  environment: string;
  status: "pending" | "approved" | "active" | "superseded" | "rolled_back";
  approved_by?: string[];
}

export interface InvokeReleaseResponse {
  id: string;
  capability_version_id: string;
  timestamp: string;
  inputs: Record<string, unknown>;
  environment: string;
  error?: string;
  outputs?: Record<string, unknown>;
}

export interface ClientConfig {
  baseUrl: string;
  apiKey?: string;
}

export class PromptsheonAPIError extends Error {
  constructor(public status: number, public method: string, public path: string, public body: unknown) {
    super(`${method} ${path} returned ${status}`);
  }
}

export class PromptsheonClient {
  constructor(private config: ClientConfig) {}

  /**
   * SDK-TS-1: returns the canonical URL prefix every method
   * builds on. Useful in tests that want to assert the route
   * shape without constructing the full request.
   */
  baseUrl(): string {
    return this.config.baseUrl;
  }

  private headers(json: boolean = false): Record<string, string> {
    const h: Record<string, string> = { Accept: "application/json" };
    if (this.config.apiKey) h.Authorization = `Bearer ${this.config.apiKey}`;
    if (json) h["Content-Type"] = "application/json";
    return h;
  }

  private async check<T>(r: Response, method: string, url: string): Promise<T | null> {
    if (!r.ok) {
      throw new PromptsheonAPIError(r.status, method, url, await r.text());
    }
    if (r.status === 204 || r.headers.get("content-length") === "0") return null;
    return (await r.json()) as T;
  }

  // --- Capabilities -----------------------------------------------------
  async listCapabilities(projectId: string): Promise<Capability[]> {
    const url = `${this.config.baseUrl}/api/v1/projects/${encodeURIComponent(projectId)}/capabilities`;
    return (await this.check<Capability[]>(await fetch(url, { headers: this.headers() }), "GET", url)) ?? [];
  }

  async getCapability(id: string): Promise<Capability> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(id)}`;
    return (await this.check<Capability>(await fetch(url, { headers: this.headers() }), "GET", url))!;
  }

  async updateCapabilityContract(id: string, contract: CapabilityContract): Promise<CapabilityContract> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(id)}/contract`;
    return (await this.check<CapabilityContract>(await fetch(url, {
      method: "PUT",
      headers: this.headers(true),
      body: JSON.stringify(contract),
    }), "PUT", url))!;
  }

  async getCapabilityContract(id: string): Promise<CapabilityContract> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(id)}/contract`;
    return (await this.check<CapabilityContract>(await fetch(url, { headers: this.headers() }), "GET", url))!;
  }

  async getCapabilityReputation(id: string): Promise<Reputation> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(id)}/reputation`;
    return (await this.check<Reputation>(await fetch(url, { headers: this.headers() }), "GET", url))!;
  }

  async diffCapabilityVersions(id: string, fromVersion: number, toVersion: number): Promise<ManifestDiff> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(id)}/diff?from=${fromVersion}&to=${toVersion}`;
    return (await this.check<ManifestDiff>(await fetch(url, { headers: this.headers() }), "GET", url))!;
  }

  async catalogSearch(workspaceId: string, query: string = "", limit: number = 100): Promise<Capability[]> {
    const params = new URLSearchParams({ workspace_id: workspaceId });
    if (query) params.set("q", query);
    if (limit) params.set("limit", String(limit));
    const url = `${this.config.baseUrl}/api/v1/catalog/capabilities?${params.toString()}`;
    return (await this.check<Capability[]>(await fetch(url, { headers: this.headers() }), "GET", url)) ?? [];
  }

  // --- Releases ---------------------------------------------------------
  async listReleases(capabilityId: string): Promise<Release[]> {
    const url = `${this.config.baseUrl}/api/v1/capabilities/${encodeURIComponent(capabilityId)}/releases`;
    return (await this.check<Release[]>(await fetch(url, { headers: this.headers() }), "GET", url)) ?? [];
  }

  async createRelease(versionId: string, environment: string = "prod"): Promise<Release> {
    const url = `${this.config.baseUrl}/api/v1/versions/${encodeURIComponent(versionId)}/releases`;
    return (await this.check<Release>(await fetch(url, {
      method: "POST",
      headers: this.headers(true),
      body: JSON.stringify({ environment }),
    }), "POST", url))!;
  }

  async voteRelease(releaseId: string, identity: string, decision: "approve" | "reject"): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/votes`;
    return await this.check(await fetch(url, {
      method: "POST",
      headers: this.headers(true),
      body: JSON.stringify({ identity, decision }),
    }), "POST", url);
  }

  async activateRelease(releaseId: string): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/activate`;
    return await this.check(await fetch(url, { method: "POST", headers: this.headers() }), "POST", url);
  }

  async rollbackRelease(releaseId: string): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/rollback`;
    return await this.check(await fetch(url, { method: "POST", headers: this.headers() }), "POST", url);
  }

  async invokeRelease(releaseId: string, body: { inputs: Record<string, unknown> }): Promise<InvokeReleaseResponse> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/invoke`;
    return (await this.check<InvokeReleaseResponse>(await fetch(url, {
      method: "POST",
      headers: this.headers(true),
      body: JSON.stringify(body),
    }), "POST", url))!;
  }

  // --- Harness ----------------------------------------------------------
  async runEval(releaseId: string, datasetId: string, scorer: string = "exact_match"): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/evals`;
    return await this.check(await fetch(url, {
      method: "POST",
      headers: this.headers(true),
      body: JSON.stringify({ dataset_id: datasetId, scorer }),
    }), "POST", url);
  }

  async reasoningCompile(intent: Record<string, unknown>): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/reasoning/compile`;
    return await this.check(await fetch(url, {
      method: "POST",
      headers: this.headers(true),
      body: JSON.stringify(intent),
    }), "POST", url);
  }

  // --- Audit / Settings ------------------------------------------------
  async verifyAuditChain(): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/audit/verify`;
    return await this.check(await fetch(url, { headers: this.headers() }), "GET", url);
  }

  async listSettings(): Promise<unknown[]> {
    const url = `${this.config.baseUrl}/api/v1/settings`;
    return (await this.check<unknown[]>(await fetch(url, { headers: this.headers() }), "GET", url)) ?? [];
  }

  async setSetting(key: string, value: string, updatedBy: string = "typescript-sdk"): Promise<unknown> {
    const url = `${this.config.baseUrl}/api/v1/settings/${encodeURIComponent(key)}`;
    return await this.check(await fetch(url, {
      method: "PUT",
      headers: this.headers(true),
      body: JSON.stringify({ value, updated_by: updatedBy }),
    }), "PUT", url);
  }
}
