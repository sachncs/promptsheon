import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { Release } from '@promptsheon/shared';

export class ReleaseRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Release | null {
    return this.db.prepare('SELECT * FROM releases WHERE id = ?').get(id) as Release | null;
  }

  findByCapabilityId(capabilityId: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ? ORDER BY created_at DESC')
      .all(capabilityId) as Release[];
  }

  findActive(capabilityId: string, environment: string): Release | null {
    return this.db.prepare("SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'")
      .get(capabilityId, environment) as Release | null;
  }

  findByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ? AND environment = ? ORDER BY created_at DESC')
      .all(capabilityId, environment) as Release[];
  }

  findMany(opts: { page: number; pageSize: number }): { items: Release[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM releases').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM releases LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Release[];
    return { items, total };
  }

  create(data: {
    capabilityId: string; capabilityVersion: number; capabilityVersionId: string | null;
    manifest: string; environment: string; createdBy?: string; canaryPercent?: number;
  }): Release {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO releases (id, capability_id, capability_version, capability_version_id, manifest, environment, status, created_at, created_by, canary_percent)
      VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?)
    `).run(id, data.capabilityId, data.capabilityVersion, data.capabilityVersionId, data.manifest, data.environment, now, data.createdBy ?? '', data.canaryPercent ?? 0);
    return {
      id, capabilityId: data.capabilityId, capabilityVersion: data.capabilityVersion,
      capabilityVersionId: data.capabilityVersionId, manifest: data.manifest,
      environment: data.environment as Release['environment'], status: 'pending',
      approvedBy: '[]', supersededBy: null, replacesReleaseId: null, createdAt: now,
      createdBy: data.createdBy ?? '', activatedAt: null, supersededAt: null,
      canaryPercent: data.canaryPercent ?? 0,
    };
  }

  updateStatus(id: string, status: Release['status']): Release | null {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    const now = new Date().toISOString();
    const activatedAt = status === 'active' ? now : existing.activatedAt;
    const supersededAt = status === 'superseded' ? now : existing.supersededAt;
    this.db.prepare('UPDATE releases SET status = ?, activated_at = ?, superseded_at = ? WHERE id = ?')
      .run(status, activatedAt, supersededAt, id);
    return { ...existing, status, activatedAt, supersededAt };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM releases WHERE id = ?').run(id);
    return true;
  }
}
