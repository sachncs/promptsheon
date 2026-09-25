import type { Schedule } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { z } from 'zod';
import { BaseRepo, type Paginated } from './base.js';
import { nextCronFire } from '../scheduler/cron.js';

const ScheduleRowSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  release_id: z.string(),
  kind: z.string(),
  cron: z.string(),
  webhook_path: z.string(),
  next_fire_at: z.string(),
  last_fire_at: z.string().nullable(),
  fired_count: z.number().int().nonnegative(),
  enabled: z.union([z.number().int(), z.boolean()]),
  created_at: z.string(),
  created_by: z.string(),
});

function toSchedule(row: unknown): Schedule {
  const value = ScheduleRowSchema.parse(row);
  return {
    id: value.id,
    workspaceId: value.workspace_id,
    releaseId: value.release_id,
    kind: value.kind,
    cron: value.cron,
    webhookPath: value.webhook_path,
    nextFireAt: value.next_fire_at,
    lastFireAt: value.last_fire_at,
    firedCount: value.fired_count,
    enabled: typeof value.enabled === 'boolean' ? value.enabled : value.enabled === 1,
    createdAt: value.created_at,
    createdBy: value.created_by,
  };
}

export class ScheduleRepo extends BaseRepo<Schedule> {
  constructor(db: Database.Database) {
    super(db, 'schedules');
  }

  override findById(id: string): Schedule | null {
    const row = this.db.prepare('SELECT * FROM schedules WHERE id = ?').get(id);
    return row ? toSchedule(row) : null;
  }

  override findMany(opts: { page: number; pageSize: number }): Paginated<Schedule> {
    const total = z.object({ count: z.number().int().nonnegative() }).parse(
      this.db.prepare('SELECT COUNT(*) AS count FROM schedules').get(),
    ).count;
    const rows = this.db.prepare('SELECT * FROM schedules LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toSchedule), total };
  }

  findDueSchedules(now: Date): Schedule[] {
    const rows = this.db.prepare("SELECT * FROM schedules WHERE enabled = 1 AND next_fire_at <= ?")
      .all(now.toISOString());
    return rows.map(toSchedule);
  }

  findByReleaseId(releaseId: string): Schedule[] {
    const rows = this.db.prepare('SELECT * FROM schedules WHERE release_id = ?')
      .all(releaseId);
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
