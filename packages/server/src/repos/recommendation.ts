import type Database from 'better-sqlite3';
import type { Recommendation, Decision } from '@promptsheon/shared';
import { notFound } from '@promptsheon/shared';

export class RecommendationRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Recommendation | null {
    return this.db.prepare('SELECT * FROM recommendations WHERE id = ?').get(id) as Recommendation | null;
  }

  findByVersionId(versionId: string): Recommendation[] {
    return this.db.prepare('SELECT * FROM recommendations WHERE capability_version_id = ? ORDER BY created_at DESC')
      .all(versionId) as Recommendation[];
  }

  create(data: { capabilityVersionId: string; type: string; payload: string }): Recommendation {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO recommendations (id, capability_version_id, type, payload, created_at) VALUES (?, ?, ?, ?, ?)')
      .run(id, data.capabilityVersionId, data.type, data.payload, now);
    return { id, capabilityVersionId: data.capabilityVersionId, type: data.type, payload: data.payload, createdAt: now };
  }

  findDecisionById(id: string): Decision | null {
    return this.db.prepare('SELECT * FROM decisions WHERE id = ?').get(id) as Decision | null;
  }

  findDecisionByRecommendationId(recommendationId: string): Decision | null {
    return this.db.prepare('SELECT * FROM decisions WHERE recommendation_id = ?').get(recommendationId) as Decision | null;
  }

  createDecision(data: { recommendationId: string; payload: string }): Decision {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO decisions (id, recommendation_id, payload, created_at) VALUES (?, ?, ?, ?)')
      .run(id, data.recommendationId, data.payload, now);
    return { id, recommendationId: data.recommendationId, payload: data.payload, createdAt: now };
  }
}
