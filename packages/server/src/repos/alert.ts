import type { AlertRule, Alert } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class AlertRepo {
  constructor(private db: Database.Database) {}

  findRules(): AlertRule[] {
    return this.db.prepare('SELECT * FROM alert_rules').all() as AlertRule[];
  }

  findRuleById(id: string): AlertRule | null {
    return this.db.prepare('SELECT * FROM alert_rules WHERE id = ?').get(id) as AlertRule | null;
  }

  createRule(data: { name: string; type: string; severity: string; enabled?: boolean; threshold?: number; duration?: number; window?: number; config?: string }): AlertRule {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO alert_rules (id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.name, data.type, data.severity, data.enabled ? 1 : 0, data.threshold ?? 0, data.duration ?? 0, data.window ?? 0, data.config ?? '', now, now);
    return {
      id, name: data.name, type: data.type as AlertRule['type'], severity: data.severity as AlertRule['severity'],
      enabled: data.enabled ?? true, threshold: data.threshold ?? 0, duration: data.duration ?? 0,
      window: data.window ?? 0, config: data.config ?? '', createdAt: now, updatedAt: now,
    };
  }

  updateRule(id: string, data: Partial<Omit<AlertRule, 'id' | 'createdAt' | 'updatedAt'>>): AlertRule | null {
    const existing = this.findRuleById(id);
    if (!existing) return null;
    const merged = { ...existing, ...data };
    this.db.prepare(`UPDATE alert_rules SET name = ?, type = ?, severity = ?, enabled = ?, threshold = ?, duration = ?, window = ?, config = ?, updated_at = ? WHERE id = ?`)
      .run(merged.name, merged.type, merged.severity, merged.enabled ? 1 : 0, merged.threshold, merged.duration, merged.window, merged.config, new Date().toISOString(), id);
    return merged;
  }

  deleteRule(id: string): boolean {
    const result = this.db.prepare('DELETE FROM alert_rules WHERE id = ?').run(id);
    return result.changes > 0;
  }

  findAlertById(id: string): Alert | null {
    return this.db.prepare('SELECT * FROM alerts WHERE id = ?').get(id) as Alert | null;
  }

  findAlerts(status?: string): Alert[] {
    if (status) {
      return this.db.prepare('SELECT * FROM alerts WHERE status = ?').all(status) as Alert[];
    }
    return this.db.prepare('SELECT * FROM alerts').all() as Alert[];
  }

  createAlert(data: { ruleId: string | null; ruleName: string; severity: string; message: string; details?: string }): Alert {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO alerts (id, rule_id, rule_name, severity, status, message, details, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.ruleId, data.ruleName, data.severity, 'active', data.message, data.details ?? '', now);
    return {
      id, ruleId: data.ruleId, ruleName: data.ruleName, severity: data.severity as Alert['severity'],
      status: 'active', message: data.message, details: data.details ?? '',
      triggeredAt: now, resolvedAt: null, acknowledgedAt: null, acknowledgedBy: null,
    };
  }

  updateAlert(id: string, data: Partial<Pick<Alert, 'status' | 'resolvedAt' | 'acknowledgedAt' | 'acknowledgedBy'>>): Alert | null {
    const existing = this.findAlertById(id);
    if (!existing) return null;
    const merged = { ...existing, ...data };
    this.db.prepare(`UPDATE alerts SET status = ?, resolved_at = ?, acknowledged_at = ?, acknowledged_by = ? WHERE id = ?`)
      .run(merged.status, merged.resolvedAt, merged.acknowledgedAt, merged.acknowledgedBy, id);
    return merged;
  }
}