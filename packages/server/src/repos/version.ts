import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { CapabilityVersion } from '@promptsheon/shared';

export class VersionRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): CapabilityVersion | null {
    return this.db.prepare('SELECT * FROM capability_versions WHERE id = ?').get(id) as CapabilityVersion | null;
  }

  findByCapabilityId(capabilityId: string): CapabilityVersion[] {
    return this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? ORDER BY version DESC')
      .all(capabilityId) as CapabilityVersion[];
  }

  findByCapabilityAndVersion(capabilityId: string, version: number): CapabilityVersion | null {
    return this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? AND version = ?')
      .get(capabilityId, version) as CapabilityVersion | null;
  }

  findMany(opts: { page: number; pageSize: number }): { items: CapabilityVersion[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM capability_versions').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM capability_versions LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as CapabilityVersion[];
    return { items, total };
  }

  create(data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }): CapabilityVersion {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)')
      .run(id, data.capabilityId, data.version, data.manifest, data.manifestHash, now, data.createdBy ?? '');
    return { id, capabilityId: data.capabilityId, version: data.version, manifest: data.manifest, manifestHash: data.manifestHash, createdAt: now, createdBy: data.createdBy ?? '' };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM capability_versions WHERE id = ?').run(id);
    return true;
  }
}
