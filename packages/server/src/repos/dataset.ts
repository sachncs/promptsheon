import type { Dataset, DatasetCase } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class DatasetRepo extends BaseRepo<Dataset> {
  constructor(db: Database.Database) {
    super(db, 'datasets');
  }

  findByCapabilityId(capabilityId: string): Dataset[] {
    return this.db.prepare('SELECT * FROM datasets WHERE capability_id = ?')
      .all(capabilityId) as Dataset[];
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

  addCase(datasetId: string, data: { inputs: string; expected: string; description?: string }): DatasetCase {
    const id = crypto.randomUUID();
    const maxSeq = (this.db.prepare('SELECT MAX(seq) as maxSeq FROM dataset_cases WHERE dataset_id = ?').get(datasetId) as { maxSeq: number | null }).maxSeq ?? 0;
    this.db.prepare(`INSERT INTO dataset_cases (id, dataset_id, seq, inputs, expected, description) VALUES (?, ?, ?, ?, ?, ?)`)
      .run(id, datasetId, maxSeq + 1, data.inputs, data.expected, data.description ?? '');
    return { id, datasetId, seq: maxSeq + 1, inputs: data.inputs, expected: data.expected, description: data.description ?? '' };
  }

  deleteCase(id: string): boolean {
    const result = this.db.prepare('DELETE FROM dataset_cases WHERE id = ?').run(id);
    return result.changes > 0;
  }
}