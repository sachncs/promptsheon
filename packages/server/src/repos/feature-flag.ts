import type Database from 'better-sqlite3';
import type { FeatureFlag } from '@promptsheon/shared';

export class FeatureFlagRepo {
  constructor(private db: Database.Database) {}

  get(name: string): FeatureFlag | null {
    return this.db.prepare('SELECT * FROM feature_flags WHERE name = ?').get(name) as FeatureFlag | null;
  }

  setEnabled(name: string, enabled: boolean): boolean {
    this.db.prepare('UPDATE feature_flags SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?')
      .run(enabled ? 1 : 0, name);
    return true;
  }

  findMany(): FeatureFlag[] {
    return this.db.prepare('SELECT * FROM feature_flags ORDER BY name').all() as FeatureFlag[];
  }
}
