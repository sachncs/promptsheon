import type Database from 'better-sqlite3';
import {
  AgentSpecificationSchema,
  canonicalizeSpecification,
  hashAgentSpecification,
  type AgentSpecification,
  type CasStore,
} from '@promptsheon/shared';
import { NotFoundError } from '@promptsheon/shared';

export interface AgentSpecificationRecord {
  hash: string;
  workspaceId: string;
  schemaVersion: string;
  parentHash: string | null;
  author: string;
  changeReason: string;
  status: 'draft' | 'candidate' | 'published' | 'retired';
  createdAt: string;
  publishedAt: string | null;
  specification: AgentSpecification;
}

export interface SpecificationDiffEntry {
  path: string;
  before: unknown;
  after: unknown;
}

export type AgentSpecificationMetadata = Omit<AgentSpecificationRecord, 'specification'>;

interface SpecificationRow {
  hash: string;
  workspace_id: string;
  schema_version: string;
  parent_hash: string | null;
  author: string;
  change_reason: string;
  status: AgentSpecificationRecord['status'];
  created_at: string;
  published_at: string | null;
}

/** Persistence boundary for immutable specifications and their lineage. */
export class AgentSpecificationRepo {
  private readonly parsedCache = new Map<string, AgentSpecification>();

  constructor(
    private readonly db: Database.Database,
    private readonly cas: CasStore,
  ) {}

  async create(input: {
    workspaceId: string;
    specification: AgentSpecification;
    author: string;
    changeReason: string;
    parentHash?: string | null;
  }): Promise<AgentSpecificationRecord> {
    const specification = AgentSpecificationSchema.parse(input.specification);
    if (input.parentHash) {
      const parent = this.db.prepare(
        'SELECT 1 AS present FROM agent_specifications WHERE workspace_id = ? AND hash = ?',
      ).get(input.workspaceId, input.parentHash) as { present: number } | undefined;
      if (!parent) throw new NotFoundError('agent specification parent', input.parentHash);
    }
    const canonical = canonicalizeSpecification(specification);
    const hash = hashAgentSpecification(specification);
    await this.cas.writeBlob(Buffer.from(canonical, 'utf8'));
    const now = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO agent_specifications
        (hash, workspace_id, schema_version, parent_hash, author, change_reason, status, created_at, published_at)
      VALUES (?, ?, ?, ?, ?, ?, 'draft', ?, NULL)
      ON CONFLICT(workspace_id, hash) DO NOTHING
    `).run(
      hash,
      input.workspaceId,
      specification.schemaVersion,
      input.parentHash ?? null,
      input.author,
      input.changeReason,
      now,
    );
    return this.get(input.workspaceId, hash);
  }

  async get(workspaceId: string, hash: string): Promise<AgentSpecificationRecord> {
    const row = this.db.prepare(
      'SELECT * FROM agent_specifications WHERE workspace_id = ? AND hash = ?',
    ).get(workspaceId, hash) as SpecificationRow | undefined;
    if (!row) throw new NotFoundError('agent specification', hash);
    const bytes = await this.cas.readBlob(hash);
    const cached = this.parsedCache.get(hash);
    const specification = cached ?? AgentSpecificationSchema.parse(JSON.parse(bytes.toString('utf8')) as unknown);
    if (hashAgentSpecification(specification) !== hash) {
      throw new Error(`agent specification hash mismatch: ${hash}`);
    }
    if (!cached) {
      this.parsedCache.set(hash, specification);
      if (this.parsedCache.size > 1_024) {
        const oldest = this.parsedCache.keys().next().value;
        if (oldest) this.parsedCache.delete(oldest);
      }
    }
    return this.toRecord(row, specification);
  }

  /** Return workspace revisions newest first without reading their CAS payloads. */
  list(
    workspaceId: string,
    options: { page?: number; pageSize?: number; status?: AgentSpecificationRecord['status'] } = {},
  ): { items: AgentSpecificationMetadata[]; total: number } {
    const page = options.page ?? 1;
    const pageSize = Math.min(options.pageSize ?? 20, 100);
    const conditions = ['workspace_id = ?'];
    const parameters: unknown[] = [workspaceId];
    if (options.status) {
      conditions.push('status = ?');
      parameters.push(options.status);
    }
    const where = `WHERE ${conditions.join(' AND ')}`;
    const total = (this.db.prepare(`SELECT COUNT(*) AS count FROM agent_specifications ${where}`).get(...parameters) as { count: number }).count;
    const rows = this.db.prepare(`
      SELECT * FROM agent_specifications ${where}
      ORDER BY created_at DESC, hash DESC
      LIMIT ? OFFSET ?
    `).all(...parameters, pageSize, (page - 1) * pageSize) as SpecificationRow[];
    return { items: rows.map((row) => this.toRecordMetadata(row)), total };
  }

  async diff(workspaceId: string, leftHash: string, rightHash: string): Promise<SpecificationDiffEntry[]> {
    const [left, right] = await Promise.all([this.get(workspaceId, leftHash), this.get(workspaceId, rightHash)]);
    const changes: SpecificationDiffEntry[] = [];
    collectDiff(left.specification, right.specification, '', changes);
    return changes;
  }

  listLineage(workspaceId: string, hash: string): Array<Omit<AgentSpecificationRecord, 'specification'>> {
    const rows: SpecificationRow[] = [];
    const seen = new Set<string>();
    let currentHash: string | null = hash;
    while (currentHash) {
      if (seen.has(currentHash)) throw new Error(`agent specification lineage cycle: ${currentHash}`);
      seen.add(currentHash);
      const row = this.db.prepare(
        'SELECT * FROM agent_specifications WHERE workspace_id = ? AND hash = ?',
      ).get(workspaceId, currentHash) as SpecificationRow | undefined;
      if (!row) throw new NotFoundError('agent specification', currentHash);
      rows.push(row);
      currentHash = row.parent_hash;
    }
    return rows.reverse().map((row) => this.toRecordMetadata(row));
  }

  publish(workspaceId: string, hash: string): void {
    const result = this.db.prepare(`
      UPDATE agent_specifications
      SET status = 'published', published_at = COALESCE(published_at, ?)
      WHERE workspace_id = ? AND hash = ? AND status IN ('draft', 'candidate', 'published')
    `).run(new Date().toISOString(), workspaceId, hash);
    if (result.changes === 0) throw new NotFoundError('agent specification', hash);
  }

  /** Immutable revisions cannot be replaced; callers must create a new hash. */
  replace(): never {
    throw new Error('published agent specifications are immutable; create a new revision');
  }

  private toRecord(row: SpecificationRow, specification: AgentSpecification): AgentSpecificationRecord {
    return { ...this.toRecordMetadata(row), specification };
  }

  private toRecordMetadata(row: SpecificationRow): AgentSpecificationMetadata {
    return {
      hash: row.hash,
      workspaceId: row.workspace_id,
      schemaVersion: row.schema_version,
      parentHash: row.parent_hash,
      author: row.author,
      changeReason: row.change_reason,
      status: row.status,
      createdAt: row.created_at,
      publishedAt: row.published_at,
    };
  }
}

function collectDiff(left: unknown, right: unknown, path: string, changes: SpecificationDiffEntry[]): void {
  if (Object.is(left, right)) return;
  if (Array.isArray(left) && Array.isArray(right)) {
    const length = Math.max(left.length, right.length);
    for (let index = 0; index < length; index++) {
      collectDiff(left[index], right[index], `${path}/${index}`, changes);
    }
    return;
  }
  if (left && right && typeof left === 'object' && typeof right === 'object') {
    const keys = new Set([...Object.keys(left), ...Object.keys(right)]);
    for (const key of [...keys].sort()) {
      collectDiff(
        (left as Record<string, unknown>)[key],
        (right as Record<string, unknown>)[key],
        `${path}/${key}`,
        changes,
      );
    }
    return;
  }
  changes.push({ path: path || '/', before: left, after: right });
}
