import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { AlertRule, Alert } from '@promptsheon/shared';

export class AlertRepo {
  constructor(private db: Database.Database) {}

  findRuleById(id: string): AlertRule | null {
    return this.db.prepare('SELECT * FROM alert_rules WHERE id = ?').get(id) as AlertRule | null;
  }

  findRules(): AlertRule[] {
    return this.db.prepare('SELECT * FROM alert_rules ORDER BY created_at DESC').all() as AlertRule[];
  }

  createRule(data: { name: string; type: string; severity: string; enabled?: boolean; threshold?: number; duration?: number; window?: number; config?: string }): AlertRule {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO alert_rules (id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)')
      .run(id, data.name, data.type, data.severity, data.enabled !== false ? 1 : 0, data.threshold ?? 0, data.duration ?? 0, data.window ?? 0, data.config ?? null, now, now);
    return { id, name: data.name, type: data.type, severity: data.severity as AlertRule['severity'], enabled: data.enabled !== false, threshold: data.threshold ?? 0, duration: data.duration ?? 0, window: data.window ?? 0, config: data.config ?? null, createdAt: now, updatedAt: now };
  }

  updateRule(id: string, data: Partial<Omit<AlertRule, 'id' | 'createdAt' | 'updatedAt'>>): AlertRule | null {
    const existing = this.findRuleById(id);
    if (!existing) throw new NotFoundError("resource", id);
    const now = new Date().toISOString();
    this.db.prepare('UPDATE alert_rules SET name = ?, type = ?, severity = ?, enabled = ?, threshold = ?, duration = ?, window = ?, config = ?, updated_at = ? WHERE id = ?')
      .run(data.name ?? existing.name, data.type ?? existing.type, data.severity ?? existing.severity, (data.enabled ?? existing.enabled) ? 1 : 0, data.threshold ?? existing.threshold, data.duration ?? existing.duration, data.window ?? existing.window, data.config ?? existing.config, now, id);
    return { ...existing, ...data, updatedAt: now };
  }

  deleteRule(id: string): boolean {
    const existing = this.findRuleById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM alert_rules WHERE id = ?').run(id);
    return true;
  }

  findAlertById(id: string): Alert | null {
    return this.db.prepare('SELECT * FROM alerts WHERE id = ?').get(id) as Alert | null;
  }

  findAlerts(status?: string): Alert[] {
    if (status) {
      return this.db.prepare('SELECT * FROM alerts WHERE status = ? ORDER BY triggered_at DESC').all(status) as Alert[];
    }
    return this.db.prepare('SELECT * FROM alerts ORDER BY triggered_at DESC').all() as Alert[];
  }

  createAlert(data: { ruleId: string | null; ruleName: string; severity: string; message: string; details?: string }): Alert {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO alerts (id, rule_id, rule_name, severity, status, message, details, triggered_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)')
      .run(id, data.ruleId, data.ruleName, data.severity, 'active', data.message, data.details ?? null, now);
    return { id, ruleId: data.ruleId, ruleName: data.ruleName, severity: data.severity as Alert['severity'], status: 'active', message: data.message, details: data.details ?? null, triggeredAt: now, resolvedAt: null, acknowledgedAt: null, acknowledgedBy: null };
  }

  updateAlert(id: string, data: Partial<Pick<Alert, 'status' | 'resolvedAt' | 'acknowledgedAt' | 'acknowledgedBy'>>): Alert | null {
    const existing = this.findAlertById(id);
    if (!existing) throw new NotFoundError("resource", id);
    const now = new Date().toISOString();
    this.db.prepare('UPDATE alerts SET status = ?, resolved_at = ?, acknowledged_at = ?, acknowledged_by = ? WHERE id = ?')
      .run(data.status ?? existing.status, data.resolvedAt ?? existing.resolvedAt, data.acknowledgedAt ?? existing.acknowledgedAt, data.acknowledgedBy ?? existing.acknowledgedBy, id);
    return { ...existing, ...data };
  }
}
