export type AlertStatus = 'active' | 'resolved';
export type AlertSeverity = 'info' | 'warning' | 'critical';

export interface AlertRule {
  id: string;
  name: string;
  type: string;
  severity: AlertSeverity;
  enabled: boolean;
  threshold: number;
  duration: number;
  window: number;
  config: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Alert {
  id: string;
  ruleId: string | null;
  ruleName: string;
  severity: AlertSeverity;
  status: AlertStatus;
  message: string;
  details: string | null;
  triggeredAt: string;
  resolvedAt: string | null;
  acknowledgedAt: string | null;
  acknowledgedBy: string | null;
}
