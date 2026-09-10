import type Database from 'better-sqlite3';
import type { FeatureFlag } from '@promptsheon/shared';

interface FeatureFlagRow {
  name: string;
  enabled: number;
  description: string;
  value: string;
  updated_at: string;
}

function parseValue(raw: string): unknown {
  if (raw === 'null' || raw === '') return null;
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

function serializeValue(v: unknown): string {
  if (v === undefined) return 'null';
  if (v === null) return 'null';
  if (typeof v === 'string') return JSON.stringify(v);
  return JSON.stringify(v);
}

function rowToFlag(r: FeatureFlagRow): FeatureFlag {
  return {
    name: r.name,
    enabled: r.enabled === 1,
    description: r.description,
    value: parseValue(r.value),
    updatedAt: r.updated_at,
  };
}

export class FeatureFlagRepo {
  constructor(private db: Database.Database) {}

  findMany(): FeatureFlag[] {
    const rows = this.db
      .prepare('SELECT name, enabled, description, value, updated_at FROM feature_flags ORDER BY name')
      .all() as FeatureFlagRow[];
    return rows.map(rowToFlag);
  }

  find(name: string): FeatureFlag | null {
    const row = this.db
      .prepare('SELECT name, enabled, description, value, updated_at FROM feature_flags WHERE name = ?')
      .get(name) as FeatureFlagRow | undefined;
    return row ? rowToFlag(row) : null;
  }

  upsert(input: { name: string; enabled: boolean; description?: string; value?: unknown }): FeatureFlag {
    this.db
      .prepare(
        `INSERT INTO feature_flags (name, enabled, description, value, updated_at)
           VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
         ON CONFLICT(name) DO UPDATE SET
           enabled = excluded.enabled,
           description = CASE WHEN excluded.description <> '' THEN excluded.description ELSE feature_flags.description END,
           value = excluded.value,
           updated_at = CURRENT_TIMESTAMP`,
      )
      .run(input.name, input.enabled ? 1 : 0, input.description ?? '', serializeValue(input.value ?? null));
    const row = this.db
      .prepare('SELECT name, enabled, description, value, updated_at FROM feature_flags WHERE name = ?')
      .get(input.name) as FeatureFlagRow;
    return rowToFlag(row);
  }

  setEnabled(name: string, enabled: boolean): boolean {
    const result = this.db
      .prepare('UPDATE feature_flags SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?')
      .run(enabled ? 1 : 0, name);
    return result.changes > 0;
  }

  delete(name: string): boolean {
    const result = this.db.prepare('DELETE FROM feature_flags WHERE name = ?').run(name);
    return result.changes > 0;
  }
}
