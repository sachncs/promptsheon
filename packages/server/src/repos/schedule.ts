import type { Schedule } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize, type Paginated } from './base.js';
import { nextCronFire } from '../scheduler/cron.js';

function toSchedule(row: Record<string, unknown>): Schedule {
  const value = camelize(row);
  return { ...value, enabled: Boolean(value['enabled']) } as unknown as Schedule;
}

export class ScheduleRepo extends BaseRepo<Schedule> {
  constructor(db: Database.Database) {
    super(db, 'schedules');
  }

  override findById(id: string): Schedule | null {
    const schedule = super.findById(id);
    return schedule ? toSchedule(schedule as unknown as Record<string, unknown>) : null;
  }

  override findMany(opts: { page: number; pageSize: number }): Paginated<Schedule> {
    const result = super.findMany(opts);
    return { ...result, items: result.items.map((schedule) => toSchedule(schedule as unknown as Record<string, unknown>)) };
  }

  findDueSchedules(now: Date): Schedule[] {
    const rows = this.db.prepare("SELECT * FROM schedules WHERE enabled = 1 AND next_fire_at <= ?")
      .all(now.toISOString()) as Array<Record<string, unknown>>;
    return rows.map(toSchedule);
  }

  findByReleaseId(releaseId: string): Schedule[] {
    const rows = this.db.prepare('SELECT * FROM schedules WHERE release_id = ?')
      .all(releaseId) as Array<Record<string, unknown>>;
    return rows.map(toSchedule);
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
