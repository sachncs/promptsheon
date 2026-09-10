import type Database from 'better-sqlite3';
import { createHash } from 'node:crypto';
import type {
  Manifest,
  SubCapabilityManifest,
  ManifestEdge,
} from '@promptsheon/shared';
import { NotFoundError } from '@promptsheon/shared';

/**
 * Compute deterministic SHA-256 hash of a Manifest DAG.
 * Used as the CAS content-address. Stable across runs.
 */
export function computeManifestHash(manifest: Manifest): string {
  const normalized = JSON.stringify(manifest, Object.keys(manifest).sort());
  return createHash('sha256').update(normalized).digest('hex');
}

/**
 * ManifestRepo persists Manifest DAGs across manifest_dag, manifest_nodes,
 * and manifest_edges tables.
 *
 * Uses manifest_hash as the global CAS identifier. Same manifest content
 * produces same hash, so we never store duplicates.
 */
export class ManifestRepo {
  constructor(private db: Database.Database) {}

  /**
   * Compute the SHA-256 hash of a stored manifest JSON blob. Used by
   * the release activation gate to look up approval state.
   * Parses the stored JSON and re-uses the module hashing algorithm
   * (key-sorted) so both call paths produce the same hash.
   */
  computeManifestHash(manifestJson: string): string {
    const parsed = JSON.parse(manifestJson) as unknown as Parameters<typeof computeManifestHash>[0];
    return computeManifestHash(parsed);
  }

  findByHash(hash: string): Manifest | null {
    const dag = this.db
      .prepare('SELECT * FROM manifest_dag WHERE manifest_hash = ?')
      .get(hash) as DagRow | undefined;
    if (!dag) return null;
    return this.assembleManifest(dag);
  }

  findByCapabilityAndVersion(capabilityId: string, version: number): Manifest | null {
    const dag = this.db
      .prepare('SELECT * FROM manifest_dag WHERE capability_id = ? AND version = ?')
      .get(capabilityId, version) as DagRow | undefined;
    if (!dag) return null;
    return this.assembleManifest(dag);
  }

  findLatestByCapability(capabilityId: string): Manifest | null {
    const dag = this.db
      .prepare('SELECT * FROM manifest_dag WHERE capability_id = ? ORDER BY version DESC LIMIT 1')
      .get(capabilityId) as DagRow | undefined;
    if (!dag) return null;
    return this.assembleManifest(dag);
  }

  /**
   * Upsert a manifest_dag row under a caller-specified (capabilityId, version)
   * tuple with a caller-specified manifest_hash. Use this when you already
   * have the JSON-encoded manifest string and the hash, e.g. after the
   * version-create or release-create endpoints receive a payload. The row
   * being present in manifest_dag is what the maker-checker approval flow
   * queries via findByHash().
   */
  registerFromRaw(opts: {
    capabilityId: string;
    version: number;
    manifestHash: string;
    manifestJson: string;
    goal?: string;
    createdBy?: string;
  }): void {
    const now = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO manifest_dag (
        id, capability_id, version, manifest_hash, parent_manifest_hash,
        goal, goal_metrics, manifest_json, created_by, created_at
      ) VALUES (?, ?, ?, ?, NULL, ?, '{}', ?, ?, ?)
      ON CONFLICT(manifest_hash) DO UPDATE SET
        manifest_json = excluded.manifest_json,
        goal = excluded.goal,
        created_by = excluded.created_by,
        capability_id = excluded.capability_id
    `).run(
      crypto.randomUUID(),
      opts.capabilityId,
      opts.version,
      opts.manifestHash,
      opts.goal ?? '',
      opts.manifestJson,
      opts.createdBy ?? '',
      now,
    );
  }

  /**
   * Persist a Manifest DAG to manifest_dag + manifest_nodes + manifest_edges.
   * Returns the computed manifest_hash.
   *
   * Requires a fresh manifest_hash (the caller has verified uniqueness).
   */
  create(manifest: Manifest, opts: { goal?: string; parentManifestHash?: string | null; createdBy: string }): string {
    const manifestHash = computeManifestHash(manifest);
    const now = new Date().toISOString();
    let dagId = '';
    this.db.transaction(() => {
      dagId = crypto.randomUUID();
      this.db.prepare(`
        INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, parent_manifest_hash, goal, goal_metrics, manifest_json, created_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      `).run(
        dagId,
        manifest.metadata['capabilityId'] ?? 'unknown',
        manifest.version,
        manifestHash,
        opts.parentManifestHash ?? null,
        opts.goal ?? '',
        JSON.stringify({}),
        JSON.stringify(manifest),
        opts.createdBy,
        now,
      );

      for (const node of manifest.nodes) {
        this.insertNode(dagId, node);
      }

      for (const edge of manifest.edges) {
        this.insertEdge(dagId, edge);
      }
    })();
    return manifestHash;
  }

  /**
   * Append a maker-checker approval vote. Idempotent per (manifest_id, user_id):
   * re-voting overwrites the prior vote.
   */
  upsertApproval(manifestHash: string, userId: string, vote: 'approve' | 'reject', comment = ''): boolean {
    const dag = this.db
      .prepare('SELECT id FROM manifest_dag WHERE manifest_hash = ?')
      .get(manifestHash) as { id: string } | undefined;
    if (!dag) throw new NotFoundError('manifest', manifestHash);
    this.db.prepare(`
      INSERT INTO manifest_approvals (id, manifest_id, user_id, vote, comment, created_at)
      VALUES (?, ?, ?, ?, ?, ?)
      ON CONFLICT(manifest_id, user_id) DO UPDATE SET vote = excluded.vote, comment = excluded.comment
    `).run(crypto.randomUUID(), dag.id, userId, vote, comment, new Date().toISOString());
    return true;
  }

  findApprovals(manifestHash: string): Array<{ userId: string; vote: string; comment: string; createdAt: string }> {
    const rows = this.db.prepare(`
      SELECT user_id, vote, comment, created_at FROM manifest_approvals
      WHERE manifest_id = (SELECT id FROM manifest_dag WHERE manifest_hash = ?)
    `).all(manifestHash) as Array<{ user_id: string; vote: string; comment: string; created_at: string }>;
    return rows.map((r) => ({ userId: r.user_id, vote: r.vote, comment: r.comment, createdAt: r.created_at }));
  }

  countDistinctApprovers(manifestHash: string): number {
    const result = this.db.prepare(`
      SELECT COUNT(DISTINCT user_id) as c FROM manifest_approvals
      WHERE manifest_id = (SELECT id FROM manifest_dag WHERE manifest_hash = ?)
        AND vote = 'approve'
    `).get(manifestHash) as { c: number };
    return result.c;
  }

  /**
   * Record a node execution (used by ManifestGraphExecutor in Phase 3).
   */
  recordNodeRun(run: {
    manifestHash: string;
    nodeId: string;
    executionId?: string;
    startedAt: string;
    endedAt?: string;
    latencyMs?: number;
    costUsd?: number;
    promptTokens?: number;
    completionTokens?: number;
    totalTokens?: number;
    error?: string;
    status?: string;
  }): string {
    const id = crypto.randomUUID();
    this.db.prepare(`
      INSERT INTO node_runs (id, manifest_hash, node_id, execution_id, started_at, ended_at, latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens, error, status)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      id,
      run.manifestHash,
      run.nodeId,
      run.executionId ?? null,
      run.startedAt,
      run.endedAt ?? null,
      run.latencyMs !== undefined ? String(run.latencyMs) : null,
      run.costUsd ?? 0,
      run.promptTokens ?? 0,
      run.completionTokens ?? 0,
      run.totalTokens ?? 0,
      run.error ?? '',
      run.status ?? 'pending',
    );
    return id;
  }

  findNodeRunsByExecution(executionId: string): Array<{
    id: string;
    manifestHash: string;
    nodeId: string;
    latencyMs: string | null;
    costUsd: number;
    totalTokens: number;
    error: string;
    status: string;
  }> {
    const rows = this.db.prepare(`
      SELECT id, manifest_hash, node_id, latency_ms, cost_usd, total_tokens, error, status
      FROM node_runs WHERE execution_id = ?
    `).all(executionId) as Array<{
      id: string;
      manifest_hash: string;
      node_id: string;
      latency_ms: string | null;
      cost_usd: number;
      total_tokens: number;
      error: string;
      status: string;
    }>;
    return rows.map((r) => ({
      id: r.id,
      manifestHash: r.manifest_hash,
      nodeId: r.node_id,
      latencyMs: r.latency_ms,
      costUsd: r.cost_usd,
      totalTokens: r.total_tokens,
      error: r.error,
      status: r.status,
    }));
  }

  /**
   * Idempotent cutover: ensure all existing capability_versions have a
   * corresponding manifest_dag entry. Single-node fallback Manifest.
   */
  ensureCutover(opts: { createdBy: string }): CutoverReport {
    const existing = this.db.prepare(`
      SELECT cv.id, cv.capability_id, cv.version, cv.manifest, c.name, c.description
      FROM capability_versions cv
      JOIN capabilities c ON c.id = cv.capability_id
      WHERE cv.manifest_hash IS NULL OR cv.manifest_hash = ''
    `).all() as Array<{ id: string; capability_id: string; version: number; manifest: string; name: string; description: string }>;

    const report: CutoverReport = { scanned: 0, migrated: 0, skipped: 0, errors: [] };
    for (const row of existing) {
      report.scanned++;
      try {
        this.migrateLegacyVersion(row, opts.createdBy);
        report.migrated++;
      } catch (e) {
        report.errors.push({ id: row.id, error: String(e) });
        report.skipped++;
      }
    }
    return report;
  }

  private migrateLegacyVersion(
    row: { id: string; capability_id: string; version: number; manifest: string; name: string; description: string },
    createdBy: string,
  ): void {
    let legacy: Record<string, unknown>;
    try {
      legacy = JSON.parse(row.manifest) as Record<string, unknown>;
    } catch {
      legacy = { systemPrompt: '' };
    }
    const systemPrompt = typeof legacy['systemPrompt'] === 'string' ? legacy['systemPrompt'] : '';

    const nodeId = 'root';
    const node: SubCapabilityManifest = {
      id: nodeId,
      name: row.name,
      description: row.description,
      goal: `Execute ${row.name}`,
      manifest: {
        id: `${row.id}-leaf`,
        version: 1,
        prompt: { systemPrompt, userTemplate: '{{input}}' },
        model: {
          provider: typeof legacy['provider'] === 'string' ? legacy['provider'] as string : 'openai',
          modelId: typeof legacy['model'] === 'string' ? legacy['model'] as string : 'gpt-4',
          temperature: typeof legacy['temperature'] === 'number' ? legacy['temperature'] as number : 0.7,
          maxTokens: typeof legacy['maxTokens'] === 'number' ? legacy['maxTokens'] as number : 4096,
        },
        runtime: {
          timeoutMs: 30000,
          nodeTimeoutMs: 10000,
          totalTimeoutMs: 300000,
          maxRetries: 3,
          canaryPercent: 0,
          concurrencyLimit: 10,
        },
        context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
        memory: { enabled: false, type: 'stateless' },
        guardrails: { pre: [], post: [] },
        tools: [],
        mcpServers: [],
        evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
        nodes: [],
        edges: [],
        metadata: {},
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      dependsOn: [],
      preGuardrails: [],
      postGuardrails: [],
      observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
      hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
      retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 },
      conversationManager: { kind: 'sliding-window', windowSize: 20 },
      state: { enabled: false, type: 'stateless' },
      limits: {},
    };

    const manifest: Manifest = {
      id: row.id,
      version: row.version,
      prompt: { systemPrompt, userTemplate: '{{input}}' },
      model: node.manifest.model,
      runtime: node.manifest.runtime,
      context: node.manifest.context,
      memory: node.manifest.memory,
      guardrails: node.manifest.guardrails,
      tools: node.manifest.tools,
      mcpServers: node.manifest.mcpServers,
      evaluation: node.manifest.evaluation,
      nodes: [node],
      edges: [],
      metadata: { capabilityId: row.capability_id },
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    this.db.transaction(() => {
      const manifestHash = this.create(manifest, { goal: node.goal, createdBy });
      this.db.prepare('UPDATE capability_versions SET manifest_hash = ? WHERE id = ?')
      .run(manifestHash, row.id);
    })();
  }

  private insertNode(manifestId: string, node: SubCapabilityManifest): void {
    this.db.prepare(`
      INSERT INTO manifest_nodes (id, manifest_id, node_id, name, description, goal, manifest_json, depends_on, pre_guardrails, post_guardrails, observability, hooks, retry, conversation_manager, state, storage, limits)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      crypto.randomUUID(),
      manifestId,
      node.id,
      node.name,
      node.description,
      node.goal,
      JSON.stringify(node.manifest),
      JSON.stringify(node.dependsOn),
      JSON.stringify(node.preGuardrails),
      JSON.stringify(node.postGuardrails),
      JSON.stringify(node.observability),
      JSON.stringify(node.hooks),
      JSON.stringify(node.retry),
      JSON.stringify(node.conversationManager),
      JSON.stringify(node.state),
      JSON.stringify(node.storage ?? {}),
      JSON.stringify(node.limits),
    );
  }

  private insertEdge(manifestId: string, edge: ManifestEdge): void {
    this.db.prepare(`
      INSERT INTO manifest_edges (id, manifest_id, from_node, to_node, field_mapping)
      VALUES (?, ?, ?, ?, ?)
    `).run(
      crypto.randomUUID(),
      manifestId,
      edge.from,
      edge.to,
      JSON.stringify(edge.mapping),
    );
  }

  private assembleManifest(dag: DagRow): Manifest {
    const nodes = this.db
      .prepare('SELECT * FROM manifest_nodes WHERE manifest_id = ?')
      .all(dag.id) as NodeRow[];
    const edges = this.db
      .prepare('SELECT * FROM manifest_edges WHERE manifest_id = ?')
      .all(dag.id) as EdgeRow[];

    const stored: Manifest = JSON.parse(dag.manifest_json) as Manifest;
    stored.nodes = nodes.map(this.nodeRowToManifest);
    stored.edges = edges.map(this.edgeRowToManifest);
    return stored;
  }

  private nodeRowToManifest = (row: NodeRow): SubCapabilityManifest => ({
    id: row.node_id,
    name: row.name,
    description: row.description,
    goal: row.goal,
    manifest: JSON.parse(row.manifest_json) as Manifest,
    dependsOn: JSON.parse(row.depends_on) as string[],
    preGuardrails: JSON.parse(row.pre_guardrails),
    postGuardrails: JSON.parse(row.post_guardrails),
    observability: JSON.parse(row.observability),
    hooks: JSON.parse(row.hooks),
    retry: JSON.parse(row.retry),
    conversationManager: JSON.parse(row.conversation_manager),
    state: JSON.parse(row.state),
    storage: JSON.parse(row.storage),
    limits: JSON.parse(row.limits),
  });

  private edgeRowToManifest = (row: EdgeRow): ManifestEdge => ({
    from: row.from_node,
    to: row.to_node,
    mapping: JSON.parse(row.field_mapping) as Record<string, string>,
  });
}

interface DagRow {
  id: string;
  capability_id: string;
  version: number;
  manifest_hash: string;
  parent_manifest_hash: string | null;
  goal: string;
  manifest_json: string;
  approved_by: string;
  approved_at: string | null;
  created_by: string;
  created_at: string;
}

interface NodeRow {
  id: string;
  manifest_id: string;
  node_id: string;
  name: string;
  description: string;
  goal: string;
  manifest_json: string;
  depends_on: string;
  pre_guardrails: string;
  post_guardrails: string;
  observability: string;
  hooks: string;
  retry: string;
  conversation_manager: string;
  state: string;
  storage: string;
  limits: string;
}

interface EdgeRow {
  id: string;
  manifest_id: string;
  from_node: string;
  to_node: string;
  field_mapping: string;
}

export interface CutoverReport {
  scanned: number;
  migrated: number;
  skipped: number;
  errors: Array<{ id: string; error: string }>;
}