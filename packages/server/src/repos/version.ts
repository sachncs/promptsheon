import type { CapabilityVersion } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize, type Paginated } from './base.js';

function toVersion(row: Record<string, unknown>): CapabilityVersion {
  return camelize(row) as unknown as CapabilityVersion;
}

export class VersionRepo extends BaseRepo<CapabilityVersion> {
  constructor(db: Database.Database) {
    super(db, 'capability_versions');
  }

  findByCapabilityId(capabilityId: string): CapabilityVersion[] {
    return this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? ORDER BY version DESC')
      .all(capabilityId)
      .map((row) => toVersion(row as Record<string, unknown>));
  }

  findByCapabilityIdInOrg(capabilityId: string, organizationId: string): CapabilityVersion[] {
    return this.db.prepare(
      `SELECT v.* FROM capability_versions v
       JOIN capabilities c ON c.id = v.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE v.capability_id = ? AND w.org_id = ? ORDER BY v.version DESC`,
    ).all(capabilityId, organizationId).map((row) => toVersion(row as Record<string, unknown>));
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<CapabilityVersion> {
    const joins = `
      FROM capability_versions v
      JOIN capabilities c ON c.id = v.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?`;
    const total = (this.db.prepare(`SELECT COUNT(*) AS count ${joins}`).get(organizationId) as { count: number }).count;
    const rows = this.db.prepare(
      `SELECT v.* ${joins} ORDER BY v.created_at DESC LIMIT ? OFFSET ?`,
    ).all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize) as Array<Record<string, unknown>>;
    return { items: rows.map(toVersion), total };
  }

  findByIdInOrg(id: string, organizationId: string): CapabilityVersion | null {
    const row = this.db.prepare(
      `SELECT v.* FROM capability_versions v
       JOIN capabilities c ON c.id = v.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE v.id = ? AND w.org_id = ?`,
    ).get(id, organizationId) as Record<string, unknown> | undefined;
    return row ? toVersion(row) : null;
  }

  createInOrg(data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }, organizationId: string): CapabilityVersion | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at)
      SELECT ?, ?, ?, ?, ?, ?, ?
      WHERE EXISTS (
        SELECT 1 FROM capabilities c
        JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = ? AND w.org_id = ?
      )
    `).run(id, data.capabilityId, data.version, data.manifest, data.manifestHash, data.createdBy ?? '', now, data.capabilityId, organizationId);
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    const result = this.db.prepare(`
      DELETE FROM capability_versions WHERE id = ? AND EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = capability_versions.capability_id AND w.org_id = ?
      )
    `).run(id, organizationId);
    return result.changes > 0;
  }

  findByCapabilityAndVersion(capabilityId: string, version: number): CapabilityVersion | null {
    const row = this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? AND version = ?')
      .get(capabilityId, version) as Record<string, unknown> | undefined;
    return row ? toVersion(row) : null;
  }

  create(data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }): CapabilityVersion {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityId, data.version, data.manifest, data.manifestHash, data.createdBy ?? '', now);
    return { id, capabilityId: data.capabilityId, version: data.version, manifest: data.manifest, manifestHash: data.manifestHash, createdBy: data.createdBy ?? '', createdAt: now };
  }
}
