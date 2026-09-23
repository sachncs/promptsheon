import type { Schedule } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';
import { nextCronFire } from '../scheduler/cron.js';

export class ScheduleRepo extends BaseRepo<Schedule> {
  constructor(db: Database.Database) {
    super(db, 'schedules');
  }

  findDueSchedules(now: Date): Schedule[] {
    return this.db.prepare("SELECT * FROM schedules WHERE enabled = 1 AND next_fire_at <= ?")
      .all(now.toISOString()) as Schedule[];
  }

  findByReleaseId(releaseId: string): Schedule[] {
    return this.db.prepare('SELECT * FROM schedules WHERE release_id = ?')
      .all(releaseId) as Schedule[];
  }

  create(data: { workspaceId: string; releaseId: string; kind: string; cron: string; enabled?: boolean }): Schedule {
    const id = crypto.randomUUID();
    const now = new Date();
    const nowIso = now.toISOString();
    const nextFireAt = nextCronFire(data.cron, now).toISOString();
    this.db.prepare(`INSERT INTO schedules (id, workspace_id, release_id, kind, cron, enabled, created_at, updated_at, next_fire_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.workspaceId, data.releaseId, data.kind, data.cron, data.enabled ? 1 : 0, nowIso, nowIso, nextFireAt);
    return {
      id, workspaceId: data.workspaceId, releaseId: data.releaseId, kind: data.kind, cron: data.cron,
      webhookPath: '', nextFireAt, lastFireAt: null, firedCount: 0,
      enabled: data.enabled ?? true, createdAt: nowIso, createdBy: '',
    };
  }

  update(id: string, data: Partial<Pick<Schedule, 'cron' | 'enabled' | 'nextFireAt'>>): Schedule | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const cron = data.cron ?? existing.cron;
    const enabled = data.enabled ?? existing.enabled;
    const nextFireAt = data.nextFireAt ?? (data.cron ? nextCronFire(cron, new Date()).toISOString() : existing.nextFireAt);
    this.db.prepare(`UPDATE schedules SET cron = ?, enabled = ?, next_fire_at = ?, updated_at = ? WHERE id = ?`)
      .run(cron, enabled ? 1 : 0, nextFireAt, new Date().toISOString(), id);
    return { ...existing, cron, enabled, nextFireAt };
  }

  advance(id: string, firedAt: Date): Schedule | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const nextFireAt = nextCronFire(existing.cron, firedAt).toISOString();
    const lastFireAt = firedAt.toISOString();
    const firedCount = existing.firedCount + 1;
    this.db.prepare(
      `UPDATE schedules
       SET last_fire_at = ?, fired_count = ?, next_fire_at = ?, updated_at = ?
       WHERE id = ?`,
    ).run(lastFireAt, firedCount, nextFireAt, lastFireAt, id);
    return { ...existing, lastFireAt, firedCount, nextFireAt };
  }
}
