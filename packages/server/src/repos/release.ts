import type { Release } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { z } from 'zod';
import { BaseRepo, type Paginated } from './base.js';
import { computeManifestHashFromJson } from './manifest.js';

const ReleaseRowSchema = z.object({
  id: z.string(),
  capability_id: z.string(),
  capability_version: z.number().int().positive(),
  capability_version_id: z.string().nullable(),
  manifest: z.string(),
  environment: z.enum(['dev', 'staging', 'prod']),
  status: z.enum(['draft', 'review', 'approved', 'canary', 'active', 'rolled_back']),
  approved_by: z.string(),
  replaces_release_id: z.string().nullable(),
  created_at: z.string(),
  created_by: z.string(),
  activated_at: z.string().nullable(),
  canary_percent: z.number().int().min(0).max(100),
  release_signature: z.string().nullable(),
  signed_key_id: z.string().nullable(),
  signed_at: z.string().nullable(),
  promotion_proposal_id: z.string().nullable().optional(),
});

function toRelease(row: unknown): Release {
  const value = ReleaseRowSchema.parse(row);
  return {
    id: value.id,
    capabilityId: value.capability_id,
    capabilityVersion: value.capability_version,
    capabilityVersionId: value.capability_version_id,
    manifest: value.manifest,
    environment: value.environment,
    status: value.status,
    approvedBy: value.approved_by,
    replacesReleaseId: value.replaces_release_id,
    createdAt: value.created_at,
    createdBy: value.created_by,
    activatedAt: value.activated_at,
    canaryPercent: value.canary_percent,
    signature: value.release_signature,
    signedKeyId: value.signed_key_id,
    signedAt: value.signed_at,
    promotionProposalId: value.promotion_proposal_id ?? null,
  };
}

export class ReleaseRepo extends BaseRepo<Release> {
  constructor(db: Database.Database) {
    super(db, 'releases');
  }

  findByCapabilityId(capabilityId: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ?')
      .all(capabilityId)
      .map(toRelease);
  }

  findByCapabilityIdInOrg(capabilityId: string, organizationId: string): Release[] {
    return this.db.prepare(
      `SELECT r.* FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.capability_id = ? AND w.org_id = ?
       ORDER BY r.created_at DESC`,
    ).all(capabilityId, organizationId)
      .map(toRelease);
  }

  findManyInOrg(
    organizationId: string,
    opts: { page: number; pageSize: number; status?: string },
  ): Paginated<Release> {
    const joins = `
      FROM releases r
      JOIN capabilities c ON c.id = r.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?${opts.status ? ' AND r.status = ?' : ''}`;
    const params = opts.status ? [organizationId, opts.status] : [organizationId];
    const total = z.object({ count: z.number().int().nonnegative() }).parse(
      this.db.prepare(`SELECT COUNT(*) AS count ${joins}`).get(...params),
    ).count;
    const rows = this.db.prepare(
      `SELECT r.* ${joins} ORDER BY r.created_at DESC LIMIT ? OFFSET ?`,
    ).all(...params, opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toRelease), total };
  }

  findByIdInOrg(id: string, organizationId: string): Release | null {
    const row = this.db
      .prepare(
        `SELECT r.*
         FROM releases r
         JOIN capabilities c ON c.id = r.capability_id
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE r.id = ? AND w.org_id = ?`,
      )
      .get(id, organizationId);
    return row ? toRelease(row) : null;
  }

  findByPromotionProposalInOrg(proposalId: string, organizationId: string): Release | null {
    const row = this.db.prepare(
      `SELECT r.*
       FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.promotion_proposal_id = ? AND w.org_id = ?`,
    ).get(proposalId, organizationId);
    return row ? toRelease(row) : null;
  }

  createInOrg(
    data: { capabilityId: string; capabilityVersion: number; capabilityVersionId: string | null; manifest: string; environment: string; createdBy?: string; canaryPercent?: number; promotionProposalId?: string | undefined },
    organizationId: string,
  ): Release | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO releases (id, capability_id, capability_version, capability_version_id, manifest, environment, status, created_by, canary_percent, created_at, updated_at, promotion_proposal_id)
      SELECT ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?
      WHERE EXISTS (
        SELECT 1 FROM capabilities c
        JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = ? AND w.org_id = ?
      )
    `).run(
      id, data.capabilityId, data.capabilityVersion, data.capabilityVersionId,
      data.manifest, data.environment, data.createdBy ?? '', data.canaryPercent ?? 0,
      now, now, data.promotionProposalId ?? null, data.capabilityId, organizationId,
    );
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
  }

  attachSignatureInOrg(input: {
    releaseId: string;
    organizationId: string;
    signature: string;
    signedKeyId: string;
    signedAt: string;
  }): Release | null {
    const result = this.db.prepare(
      `UPDATE releases SET release_signature = ?, signed_key_id = ?, signed_at = ?, updated_at = ?
       WHERE id = ? AND EXISTS (
         SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE c.id = releases.capability_id AND w.org_id = ?
       )`,
    ).run(input.signature, input.signedKeyId, input.signedAt, new Date().toISOString(), input.releaseId, input.organizationId);
    return result.changes > 0 ? this.findByIdInOrg(input.releaseId, input.organizationId) : null;
  }

  findActive(capabilityId: string, environment: string): Release | null {
    const row = this.db.prepare("SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'")
      .get(capabilityId, environment);
    return row ? toRelease(row) : null;
  }

  findByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare('SELECT * FROM releases WHERE capability_id = ? AND environment = ?')
      .all(capabilityId, environment)
      .map(toRelease);
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
      replacesReleaseId: null, activatedAt: null,
      signature: null, signedKeyId: null, signedAt: null,
    };
  }

  updateStatus(id: string, status: Release['status']): Release | null {
    const existing = this.findById(id);
    if (!existing) return null;
    this.db.prepare(`UPDATE releases SET status = ?, updated_at = ? WHERE id = ?`)
      .run(status, new Date().toISOString(), id);
    return { ...existing, status };
  }

  updateStatusInOrg(id: string, organizationId: string, status: Release['status']): Release | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing) return null;
    this.db.prepare(`
      UPDATE releases SET status = ?, updated_at = ?
      WHERE id = ? AND EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = releases.capability_id AND w.org_id = ?
      )
    `).run(status, new Date().toISOString(), id, organizationId);
    return { ...existing, status };
  }

  updateStatusInOrgIfCurrent(id: string, organizationId: string, expected: Release['status'], status: Release['status']): Release | null {
    const result = this.db.prepare(`
      UPDATE releases SET status = ?, updated_at = ?
      WHERE id = ? AND status = ? AND EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = releases.capability_id AND w.org_id = ?
      )
    `).run(status, new Date().toISOString(), id, expected, organizationId);
    return result.changes > 0 ? this.findByIdInOrg(id, organizationId) : null;
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
      const currentUpdate = this.db.prepare("UPDATE releases SET status = 'rolled_back', updated_at = ? WHERE id = ? AND status IN ('canary', 'active')").run(new Date().toISOString(), currentId);
      if (currentUpdate.changes !== 1) throw new Error('current release changed before rollback');
      const targetUpdate = this.db.prepare("UPDATE releases SET status = 'active', updated_at = ? WHERE id = ? AND status IN ('active', 'rolled_back')").run(new Date().toISOString(), targetId);
      if (targetUpdate.changes !== 1) throw new Error('rollback target changed before rollback');
      rolledBack = { ...current, status: 'rolled_back' };
      reactivated = { ...target, status: 'active' };
    })();
    if (!rolledBack || !reactivated) return null;
    return { rolledBack, reactivated };
  }

  rollbackAtomicallyInOrg(
    currentId: string,
    targetId: string,
    organizationId: string,
  ): { rolledBack: Release; reactivated: Release } | null {
    const current = this.findByIdInOrg(currentId, organizationId);
    const target = this.findByIdInOrg(targetId, organizationId);
    if (!current || !target || currentId === targetId) return null;
    let result: { rolledBack: Release; reactivated: Release } | null = null;
    this.db.transaction(() => {
      const now = new Date().toISOString();
      const currentUpdate = this.db.prepare("UPDATE releases SET status = 'rolled_back', updated_at = ? WHERE id = ? AND status IN ('canary', 'active')").run(now, currentId);
      if (currentUpdate.changes !== 1) throw new Error('current release changed before rollback');
      const targetUpdate = this.db.prepare("UPDATE releases SET status = 'active', updated_at = ? WHERE id = ? AND status IN ('active', 'rolled_back')").run(now, targetId);
      if (targetUpdate.changes !== 1) throw new Error('rollback target changed before rollback');
      result = {
        rolledBack: { ...current, status: 'rolled_back' },
        reactivated: { ...target, status: 'active' },
      };
    })();
    return result;
  }

  /**
   * Compute the deterministic manifest_hash for a stored release.manifest
   * blob. Used by the activation gate to look up approval state.
   */
  computeManifestHash(manifestJson: string): string {
    return computeManifestHashFromJson(manifestJson);
  }

  findActiveByCapabilityAndEnv(capabilityId: string, environment: string): Release[] {
    return this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'active'",
    ).all(capabilityId, environment).map(toRelease);
  }

  findActiveByManifestHash(manifestHash: string): Release[] {
    const all = this.db.prepare(
      "SELECT * FROM releases WHERE status = 'active'",
    ).all().map(toRelease);
    return all.filter((release) => {
      try {
        return this.computeManifestHash(release.manifest) === manifestHash;
      } catch {
        return false;
      }
    });
  }

  findActiveByManifestHashInOrg(manifestHash: string, organizationId: string): Release[] {
    return this.db.prepare(
      `SELECT r.* FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.status = 'active' AND w.org_id = ?`,
    ).all(organizationId)
      .map(toRelease)
      .filter((release) => {
        try {
          return this.computeManifestHash(release.manifest) === manifestHash;
        } catch {
          return false;
        }
      });
  }

  /** Find the most recent rolled-back release for a capability and environment. */
  findPreviousActive(capabilityId: string, environment: string, currentVersion: number): Release | null {
    const row = this.db.prepare(
      "SELECT * FROM releases WHERE capability_id = ? AND environment = ? AND status = 'rolled_back' AND capability_version < ? ORDER BY capability_version DESC LIMIT 1",
    ).get(capabilityId, environment, currentVersion);
    return row ? toRelease(row) : null;
  }

  findPreviousActiveInOrg(capabilityId: string, environment: string, currentVersion: number, organizationId: string): Release | null {
    const row = this.db.prepare(
      `SELECT r.* FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.capability_id = ? AND r.environment = ?
         AND r.status = 'rolled_back'
         AND r.capability_version < ? AND w.org_id = ?
       ORDER BY r.capability_version DESC LIMIT 1`,
    ).get(capabilityId, environment, currentVersion, organizationId);
    return row ? toRelease(row) : null;
  }

  /** Return the current active peer used as the deterministic rollback target. */
  findActivePeerInOrg(current: Release, organizationId: string): Release | null {
    const row = this.db.prepare(
      `SELECT r.* FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.capability_id = ? AND r.environment = ? AND r.status = 'active'
         AND r.id <> ? AND w.org_id = ?
       ORDER BY r.created_at DESC LIMIT 1`,
    ).get(current.capabilityId, current.environment, current.id, organizationId);
    return row ? toRelease(row) : null;
  }

  listCanaryReleases(): Array<{ releaseId: string; organizationId: string }> {
    const rows = this.db.prepare(
      `SELECT r.id AS release_id, w.org_id AS organization_id
       FROM releases r
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE r.status = 'canary' ORDER BY r.created_at ASC`,
    ).all() as Array<{ release_id: string; organization_id: string }>;
    return rows.map((row) => ({ releaseId: row.release_id, organizationId: row.organization_id }));
  }

  updateCanaryPercent(id: string, percent: number): Release | null {
    const existing = this.findById(id);
    if (!existing) return null;
    this.db.prepare(`UPDATE releases SET canary_percent = ?, updated_at = ? WHERE id = ?`)
      .run(percent, new Date().toISOString(), id);
    return { ...existing, canaryPercent: percent };
  }

  updateCanaryPercentInOrg(id: string, organizationId: string, percent: number): Release | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing || (existing.status !== 'canary' && existing.status !== 'active')) return null;
    this.db.prepare(`
      UPDATE releases SET canary_percent = ?, updated_at = ?
      WHERE id = ? AND EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = releases.capability_id AND w.org_id = ?
      )
        AND status IN ('canary', 'active')
    `).run(percent, new Date().toISOString(), id, organizationId);
    return { ...existing, canaryPercent: percent };
  }

  listTransitions(releaseId: string): Array<{
    id: string;
    releaseId: string;
    fromStatus: string | null;
    toStatus: string;
    actorId: string;
    reason: string | null;
    createdAt: string;
  }> {
    const rows = this.db
      .prepare(
        'SELECT * FROM release_transitions WHERE release_id = ? ORDER BY created_at ASC',
      )
      .all(releaseId) as Array<{
        id: string;
        release_id: string;
        from_status: string | null;
        to_status: string;
        actor_id: string;
        reason: string | null;
        created_at: string;
      }>;
    return rows.map((r) => ({
      id: r.id,
      releaseId: r.release_id,
      fromStatus: r.from_status,
      toStatus: r.to_status,
      actorId: r.actor_id,
      reason: r.reason,
      createdAt: r.created_at,
    }));
  }

  appendTransition(row: {
    id: string;
    releaseId: string;
    fromStatus: string | null;
    toStatus: string;
    actorId: string;
    reason: string | null;
    createdAt: string;
  }): void {
    this.db
      .prepare(
        `INSERT INTO release_transitions (id, release_id, from_status, to_status, actor_id, reason, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(row.id, row.releaseId, row.fromStatus, row.toStatus, row.actorId, row.reason, row.createdAt);
  }
}
