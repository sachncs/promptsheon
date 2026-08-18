import type Database from 'better-sqlite3';
import type { LineageEdge } from '@promptsheon/shared';
import { notFound } from '@promptsheon/shared';

export class LineageRepo {
  constructor(private db: Database.Database) {}

  findById(id: number): LineageEdge | null {
    return this.db.prepare('SELECT * FROM lineage_edges WHERE id = ?').get(id) as LineageEdge | null;
  }

  findByCapabilityId(capabilityId: string): LineageEdge[] {
    return this.db.prepare('SELECT * FROM lineage_edges WHERE capability_id = ? ORDER BY created_at DESC')
      .all(capabilityId) as LineageEdge[];
  }

  findByParent(capabilityId: string, version: number): LineageEdge[] {
    return this.db.prepare('SELECT * FROM lineage_edges WHERE parent_capability_id = ? AND parent_version = ?')
      .all(capabilityId, version) as LineageEdge[];
  }

  findByChild(capabilityId: string, version: number): LineageEdge[] {
    return this.db.prepare('SELECT * FROM lineage_edges WHERE child_capability_id = ? AND child_version = ?')
      .all(capabilityId, version) as LineageEdge[];
  }

  create(data: {
    capabilityId: string; parentCapabilityId: string; parentVersion: number;
    childCapabilityId: string; childVersion: number; source: string;
    recommendationId?: string | null; createdBy?: string; notes?: string;
  }): LineageEdge {
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO lineage_edges (capability_id, parent_capability_id, parent_version, child_capability_id, child_version, source, recommendation_id, created_at, created_by, notes)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(data.capabilityId, data.parentCapabilityId, data.parentVersion, data.childCapabilityId, data.childVersion, data.source, data.recommendationId ?? null, now, data.createdBy ?? '', data.notes ?? '{}');
    return {
      id: Number(result.lastInsertRowid), capabilityId: data.capabilityId,
      parentCapabilityId: data.parentCapabilityId, parentVersion: data.parentVersion,
      childCapabilityId: data.childCapabilityId, childVersion: data.childVersion,
      source: data.source as LineageEdge['source'], recommendationId: data.recommendationId ?? null,
      createdAt: now, createdBy: data.createdBy ?? '', notes: data.notes ?? '{}',
    };
  }

  delete(id: number): void {
    this.db.prepare('DELETE FROM lineage_edges WHERE id = ?').run(id);
  }
}
