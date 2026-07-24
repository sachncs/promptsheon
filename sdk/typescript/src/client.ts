/**
 * TypeScript client for the Promptsheon v1 API.
 *
 * Today this file is hand-written against the public resource list
 * in the architecture review (§7). The M3 follow-on commit runs
 * `npx openapi-typescript api/openapi.yaml` to regenerate this from
 * the produced spec; today the package compiles against a stub
 * `paths` type so consumers can adopt the SDK without waiting on
 * the codegen pipeline.
 */
export interface Capability {
  id: string;
  project_id: string;
  name: string;
  description?: string;
  owner?: string;
  tags?: string[];
  created_at: string;
  updated_at: string;
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

export class PromptsheonClient {
  constructor(private config: ClientConfig) {}

  async listCapabilities(projectId: string): Promise<Capability[]> {
    const url = `${this.config.baseUrl}/api/v1/projects/${encodeURIComponent(projectId)}/capabilities`;
    const r = await fetch(url, {
      headers: this.config.apiKey
        ? { Authorization: `Bearer ${this.config.apiKey}` }
        : {},
    });
    if (!r.ok) {
      throw new Error(`listCapabilities failed: ${r.status} ${r.statusText}`);
    }
    return r.json() as Promise<Capability[]>;
  }

  async invokeRelease(releaseId: string, body: { inputs: Record<string, unknown> }): Promise<InvokeReleaseResponse> {
    const url = `${this.config.baseUrl}/api/v1/releases/${encodeURIComponent(releaseId)}/invoke`;
    const r = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(this.config.apiKey ? { Authorization: `Bearer ${this.config.apiKey}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      throw new Error(`invokeRelease failed: ${r.status} ${r.statusText}`);
    }
    return r.json() as Promise<InvokeReleaseResponse>;
  }
}
