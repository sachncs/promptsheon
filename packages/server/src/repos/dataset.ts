import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { Dataset, DatasetCase } from '@promptsheon/shared';

export class DatasetRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Dataset | null {
    return this.db.prepare('SELECT * FROM datasets WHERE id = ?').get(id) as Dataset | null;
  }

  findByCapabilityId(capabilityId: string): Dataset[] {
    return this.db.prepare('SELECT * FROM datasets WHERE capability_id = ? ORDER BY created_at DESC')
      .all(capabilityId) as Dataset[];
  }

  findMany(opts: { page: number; pageSize: number }): { items: Dataset[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM datasets').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM datasets LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Dataset[];
    return { items, total };
  }

  create(data: { capabilityId: string; name: string; description?: string }): Dataset {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO datasets (id, capability_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.capabilityId, data.name, data.description ?? '', now, now);
    return { id, capabilityId: data.capabilityId, name: data.name, description: data.description ?? '', createdAt: now, updatedAt: now };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM datasets WHERE id = ?').run(id);
    return true;
  }

  findCases(datasetId: string): DatasetCase[] {
    return this.db.prepare('SELECT * FROM dataset_cases WHERE dataset_id = ? ORDER BY seq')
      .all(datasetId) as DatasetCase[];
  }

  addCase(datasetId: string, data: { inputs: string; expected: string; description?: string }): DatasetCase {
    const id = crypto.randomUUID();
    const maxSeq = (this.db.prepare('SELECT MAX(seq) as maxSeq FROM dataset_cases WHERE dataset_id = ?').get(datasetId) as { maxSeq: number | null }).maxSeq ?? 0;
    this.db.prepare('INSERT INTO dataset_cases (id, dataset_id, seq, inputs, expected, description) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, datasetId, maxSeq + 1, data.inputs, data.expected, data.description ?? '');
    return { id, datasetId, seq: maxSeq + 1, inputs: data.inputs, expected: data.expected, description: data.description ?? '' };
  }

  deleteCase(id: string): boolean {
    const result = this.db.prepare('DELETE FROM dataset_cases WHERE id = ?').run(id);
    return result.changes > 0;
  }
}
