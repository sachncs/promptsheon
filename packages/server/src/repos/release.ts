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
      .run(id, data.capabilityId, data.capabilityVersion, data.capabilityVersionId, data.manifest, data.environment, 'draft', data.createdBy ?? '', data.canaryPercent ?? 0, now, now);
    return {
      id, capabilityId: data.capabilityId, capabilityVersion: data.capabilityVersion,
      capabilityVersionId: data.capabilityVersionId, manifest: data.manifest,
      environment: data.environment as Release['environment'], status: 'draft', createdBy: data.createdBy ?? '',
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

  /**
   * Atomically rollback: roll back the current release and reactivate
   * the target in a single transaction. The UNIQUE(active-per-cap-env)
   * constraint is satisfied by rolling back first.
   *
   * On success returns `{ rolledBack, reactivated }`. On any failure
   * the entire transaction is rolled back and the pair is unchanged.
   */
  rollbackAtomically(
    currentId: string,
    targetId: string,
  ): { rolledBack: Release; reactivated: Release } | null {
    const current = this.findById(currentId);
    const target = this.findById(targetId);
    if (!current || !target) return null;
    if (currentId === targetId) return null;

    let rolledBack: Release | null = null;
    let reactivated: Release | null = null;
    this.db.transaction(() => {
      this.db.prepare(
        "UPDATE releases SET status = 'rolled_back', updated_at = ? WHERE id = ?",
      ).run(new Date().toISOString(), currentId);
      this.db.prepare(
        "UPDATE releases SET status = 'active', updated_at = ? WHERE id = ?",
      ).run(new Date().toISOString(), targetId);
      rolledBack = { ...current, status: 'rolled_back' };
      reactivated = { ...target, status: 'active' };
    })();
    if (!rolledBack || !reactivated) return null;
    return { rolledBack, reactivated };
  }

  /**
   * Compute the deterministic manifest_hash for a stored release.manifest
   * blob. Used by the activation gate to look up approval state.
   */
  computeManifestHash(manifestJson: string): string {
    const { createHash } = require('node:crypto') as typeof import('node:crypto');
    return createHash('sha256').update(manifestJson).digest('hex');
  }

  findActiveByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'",
    ).all(capabilityId, environment) as Release[];
  }

  findActiveByManifestHash(manifestHash: string): Release[] {
    const all = this.db.prepare(
      "SELECT * FROM releases WHERE status = 'active'",
    ).all() as Release[];
    return all.filter((r) => {
      try {
        const obj = JSON.parse(r.manifest) as Record<string, unknown>;
        if (obj['manifestHash'] === manifestHash) return true;
      } catch { /* ignore */ }
      const { createHash } = require('node:crypto') as typeof import('node:crypto');
      const h = createHash('sha256').update(r.manifest).digest('hex');
      return h === manifestHash;
    });
  }

  /**
   * Find the most recent rolled-back release for a (capability, env) pair
   * with capability_version < currentVersion. Used by rollback to find
   * the previous known-good release.
   */
  findPreviousActive(capabilityId: string, environment: string, currentVersion: number): Release | null {
    return this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'rolled_back' AND capability_version < ? ORDER BY capability_version DESC LIMIT 1",
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