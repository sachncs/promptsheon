import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { ScheduleRepo } from '../src/repos/schedule.js';

describe('ScheduleRepo read models', () => {
  it('maps due schedules into the camelCase domain shape', () => {
    const db = new Database(':memory:');
    db.exec(`
      CREATE TABLE schedules (
        id TEXT PRIMARY KEY,
        workspace_id TEXT NOT NULL,
        release_id TEXT NOT NULL,
        kind TEXT NOT NULL,
        cron TEXT NOT NULL,
        webhook_path TEXT NOT NULL,
        next_fire_at TEXT NOT NULL,
        last_fire_at TEXT,
        fired_count INTEGER NOT NULL,
        enabled INTEGER NOT NULL,
        created_at TEXT NOT NULL,
        created_by TEXT NOT NULL
      )
    `);
    db.prepare(
      `INSERT INTO schedules
       (id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by)
       VALUES ('schedule-1', 'workspace-1', 'release-1', 'eval', '* * * * *', '/hook', '2020-01-01T00:00:00.000Z', NULL, 3, 1, '2020-01-01T00:00:00.000Z', 'user-1')`,
    ).run();

    const [schedule] = new ScheduleRepo(db).findDueSchedules(new Date('2020-01-01T00:01:00.000Z'));
    expect(schedule).toMatchObject({
      id: 'schedule-1',
      workspaceId: 'workspace-1',
      releaseId: 'release-1',
      webhookPath: '/hook',
      nextFireAt: '2020-01-01T00:00:00.000Z',
      firedCount: 3,
      enabled: true,
    });
    db.close();
  });
});
