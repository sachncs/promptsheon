import type Database from 'better-sqlite3';
import type { NotificationGroup } from '@promptsheon/shared';
import { notFound, conflict } from '@promptsheon/shared';

export class NotificationGroupRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): NotificationGroup | null {
    return this.db.prepare('SELECT * FROM notification_groups WHERE id = ?').get(id) as NotificationGroup | null;
  }

  findMany(): NotificationGroup[] {
    return this.db.prepare('SELECT * FROM notification_groups ORDER BY name').all() as NotificationGroup[];
  }

  create(data: { name: string; channels: string }): NotificationGroup {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO notification_groups (id, name, channels, created_at) VALUES (?, ?, ?, ?)')
      .run(id, data.name, data.channels, now);
    return { id, name: data.name, channels: data.channels, createdAt: now };
  }

  delete(id: string): void {
    const existing = this.findById(id);
    if (!existing) throw notFound('notification_group', id);
    this.db.prepare('DELETE FROM notification_groups WHERE id = ?').run(id);
  }

  findByAlertRuleId(ruleId: string): string[] {
    const rows = this.db.prepare(`
      SELECT ng.channels FROM notification_groups ng
      JOIN alert_rule_notification_groups arng ON ng.id = arng.notification_group_id
      WHERE arng.alert_rule_id = ?
    `).all(ruleId) as { channels: string }[];
    return rows.flatMap((r) => JSON.parse(r.channels) as string[]);
  }

  linkRuleToGroup(ruleId: string, groupId: string): void {
    this.db.prepare('INSERT OR IGNORE INTO alert_rule_notification_groups (alert_rule_id, notification_group_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)')
      .run(ruleId, groupId);
  }

  unlinkRuleFromGroup(ruleId: string, groupId: string): void {
    this.db.prepare('DELETE FROM alert_rule_notification_groups WHERE alert_rule_id = ? AND notification_group_id = ?')
      .run(ruleId, groupId);
  }
}
