import type { Dataset, DatasetCase } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize, type Paginated } from './base.js';

export class DatasetRepo extends BaseRepo<Dataset> {
  constructor(db: Database.Database) {
    super(db, 'datasets');
  }

  findByCapabilityId(capabilityId: string): Dataset[] {
    return this.db.prepare('SELECT * FROM datasets WHERE capability_id = ?')
      .all(capabilityId) as Dataset[];
  }

  findByCapabilityIdInOrg(capabilityId: string, organizationId: string): Dataset[] {
    return this.db.prepare(
      `SELECT d.* FROM datasets d
       JOIN capabilities c ON c.id = d.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE d.capability_id = ? AND w.org_id = ? ORDER BY d.created_at DESC`,
    ).all(capabilityId, organizationId).map((row) => camelize(row as Record<string, unknown>) as unknown as Dataset);
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<Dataset> {
    const joins = `
      FROM datasets d
      JOIN capabilities c ON c.id = d.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?`;
    const total = (this.db.prepare(`SELECT COUNT(*) AS count ${joins}`).get(organizationId) as { count: number }).count;
    const rows = this.db.prepare(`SELECT d.* ${joins} ORDER BY d.created_at DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize) as Array<Record<string, unknown>>;
    return { items: rows.map((row) => camelize(row) as unknown as Dataset), total };
  }

  findByIdInOrg(id: string, organizationId: string): Dataset | null {
    const row = this.db.prepare(
      `SELECT d.* FROM datasets d
       JOIN capabilities c ON c.id = d.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE d.id = ? AND w.org_id = ?`,
    ).get(id, organizationId) as Record<string, unknown> | undefined;
    return row ? camelize(row) as unknown as Dataset : null;
  }

  createInOrg(data: { capabilityId: string; name: string; description?: string }, organizationId: string): Dataset | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO datasets (id, capability_id, name, description, created_at, updated_at)
      SELECT ?, ?, ?, ?, ?, ?
      WHERE EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = ? AND w.org_id = ?
      )
    `).run(id, data.capabilityId, data.name, data.description ?? '', now, now, data.capabilityId, organizationId);
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
  }

  create(data: { capabilityId: string; name: string; description?: string }): Dataset {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO datasets (id, capability_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityId, data.name, data.description ?? '', now, now);
    return { id, capabilityId: data.capabilityId, name: data.name, description: data.description ?? '', createdAt: now, updatedAt: now };
  }

  findCases(datasetId: string): DatasetCase[] {
    return this.db.prepare('SELECT * FROM dataset_cases WHERE dataset_id = ? ORDER BY seq')
      .all(datasetId) as DatasetCase[];
  }

  findCasesInOrg(datasetId: string, organizationId: string): DatasetCase[] | null {
    if (!this.findByIdInOrg(datasetId, organizationId)) return null;
    return this.findCases(datasetId);
  }

  addCase(datasetId: string, data: { inputs: string; expected: string; description?: string }): DatasetCase {
    const id = crypto.randomUUID();
    const maxSeq = (this.db.prepare('SELECT MAX(seq) as maxSeq FROM dataset_cases WHERE dataset_id = ?').get(datasetId) as { maxSeq: number | null }).maxSeq ?? 0;
    this.db.prepare(`INSERT INTO dataset_cases (id, dataset_id, seq, inputs, expected, description) VALUES (?, ?, ?, ?, ?, ?)`)
      .run(id, datasetId, maxSeq + 1, data.inputs, data.expected, data.description ?? '');
    return { id, datasetId, seq: maxSeq + 1, inputs: data.inputs, expected: data.expected, description: data.description ?? '' };
  }

  addCaseInOrg(datasetId: string, organizationId: string, data: { inputs: string; expected: string; description?: string }): DatasetCase | null {
    if (!this.findByIdInOrg(datasetId, organizationId)) return null;
    return this.addCase(datasetId, data);
  }

  deleteCase(id: string): boolean {
    const result = this.db.prepare('DELETE FROM dataset_cases WHERE id = ?').run(id);
    return result.changes > 0;
  }

  deleteCaseInOrg(id: string, datasetId: string, organizationId: string): boolean {
    if (!this.findByIdInOrg(datasetId, organizationId)) return false;
    const result = this.db.prepare('DELETE FROM dataset_cases WHERE id = ? AND dataset_id = ?').run(id, datasetId);
    return result.changes > 0;
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    if (!this.findByIdInOrg(id, organizationId)) return false;
    const result = this.db.prepare('DELETE FROM datasets WHERE id = ?').run(id);
    return result.changes > 0;
  }
}
