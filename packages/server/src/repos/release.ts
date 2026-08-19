import type { Release } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class ReleaseRepo extends BaseRepo<Release> {
  constructor(db: Database.Database) {
    super(db, 'releases');
  }

  findByCapabilityId(capabilityId: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ?')
      .all(capabilityId) as Release[];
  }

  findActive(capabilityId: string, environment: string): Release | null {
    return this.db.prepare("SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'")
      .get(capabilityId, environment) as Release | null;
  }

  findByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ? AND environment = ?')
      .all(capabilityId, environment) as Release[];
  }

  create(data: { capabilityId: string; capabilityVersion: number; capabilityVersionId: string | null; manifest: string; environment: string; createdBy?: string; canaryPercent?: number }): Release {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO releases (id, capability_id, capability_version, capability_version_id, manifest, environment, status, created_by, canary_percent, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityId, data.capabilityVersion, data.capabilityVersionId, data.manifest, data.environment, 'pending', data.createdBy ?? '', data.canaryPercent ?? 0, now, now);
    return {
      id, capabilityId: data.capabilityId, capabilityVersion: data.capabilityVersion,
      capabilityVersionId: data.capabilityVersionId, manifest: data.manifest,
      environment: data.environment as Release['environment'], status: 'pending', createdBy: data.createdBy ?? '',
      approvedBy: '', canaryPercent: data.canaryPercent ?? 0, createdAt: now,
      replacesReleaseId: null, activatedAt: null, supersededAt: null, supersededBy: null,
    };
  }

  updateStatus(id: string, status: Release['status']): Release | null {
    const existing = this.findById(id);
    if (!existing) return null;
    this.db.prepare(`UPDATE releases SET status = ?, updated_at = ? WHERE id = ?`)
      .run(status, new Date().toISOString(), id);
    return { ...existing, status };
  }

  findActiveByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'",
    ).all(capabilityId, environment) as Release[];
  }

  /**
   * Find the most recent superseded release for a (capability, env) pair
   * with capability_version < currentVersion. Used by rollback to find
   * the previous known-good release.
   */
  findPreviousActive(capabilityId: string, environment: string, currentVersion: number): Release | null {
    return this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'superseded' AND capability_version < ? ORDER BY capability_version DESC LIMIT 1",
    ).get(capabilityId, environment, currentVersion) as Release | null;
  }

  updateCanaryPercent(id: string, percent: number): Release | null {
    const existing = this.findById(id);
    if (!existing) return null;
    this.db.prepare(`UPDATE releases SET canary_percent = ?, updated_at = ? WHERE id = ?`)
      .run(percent, new Date().toISOString(), id);
    return { ...existing, canaryPercent: percent };
  }
}