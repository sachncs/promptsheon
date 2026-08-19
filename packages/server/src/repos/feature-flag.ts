import type { FeatureFlag } from '@promptsheon/shared';
import type Database from 'better-sqlite3';

export class FeatureFlagRepo {
  constructor(private db: Database.Database) {}

  findMany(): FeatureFlag[] {
    return this.db.prepare('SELECT * FROM feature_flags ORDER BY name').all() as FeatureFlag[];
  }

  setEnabled(name: string, enabled: boolean): boolean {
    const result = this.db.prepare('UPDATE feature_flags SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?')
      .run(enabled ? 1 : 0, name);
    return result.changes > 0;
  }
}