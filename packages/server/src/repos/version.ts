import type { CapabilityVersion } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class VersionRepo extends BaseRepo<CapabilityVersion> {
  constructor(db: Database.Database) {
    super(db, 'capability_versions');
  }

  findByCapabilityId(capabilityId: string): CapabilityVersion[] {
    return this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? ORDER BY version DESC')
      .all(capabilityId) as CapabilityVersion[];
  }

  findByCapabilityAndVersion(capabilityId: string, version: number): CapabilityVersion | null {
    return this.db.prepare('SELECT * FROM capability_versions WHERE capability_id = ? AND version = ?')
      .get(capabilityId, version) as CapabilityVersion | null;
  }

  create(data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }): CapabilityVersion {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityId, data.version, data.manifest, data.manifestHash, data.createdBy ?? '', now);
    return { id, capabilityId: data.capabilityId, version: data.version, manifest: data.manifest, manifestHash: data.manifestHash, createdBy: data.createdBy ?? '', createdAt: now };
  }
}