import type { AlertRepo } from '../repos/alert.js';
import type { ExecutionRepo } from '../repos/execution.js';
import type { SseHub } from '../sse/hub.js';

interface AlertRuleConfig {
  type: 'latency_p95' | 'error_rate' | 'cost_threshold' | 'score_drop';
  threshold: number;
  window: number;
}

interface AlertRule {
  id: string;
  name: string;
  config: string;
  enabled: boolean;
}

export class AlertChecker {
  constructor(
    private alertRepo: AlertRepo,
    private executionRepo: ExecutionRepo,
    private sseHub: SseHub,
  ) {}

  async check(rule: AlertRule): Promise<void> {
    if (!rule.enabled) return;

    const config: AlertRuleConfig = JSON.parse(rule.config ?? '{}');
    const recentExecutions = this.executionRepo.findRecent(rule.id);

    let triggered = false;
    let message = '';

    switch (config.type) {
      case 'latency_p95': {
        const latencies = recentExecutions.map((e: { latencyMs: number }) => e.latencyMs).sort((a: number, b: number) => a - b);
        const p95Index = Math.floor(latencies.length * 0.95);
        const p95 = latencies[p95Index] ?? 0;
        if (p95 > config.threshold) {
          triggered = true;
          message = `P95 latency ${p95}ms exceeds threshold ${config.threshold}ms`;
        }
        break;
      }
      case 'error_rate': {
        const errors = recentExecutions.filter((e: { error: string }) => e.error).length;
        const rate = recentExecutions.length > 0 ? errors / recentExecutions.length : 0;
        if (rate > config.threshold) {
          triggered = true;
          message = `Error rate ${(rate * 100).toFixed(1)}% exceeds threshold ${(config.threshold * 100).toFixed(1)}%`;
        }
        break;
      }
      case 'cost_threshold': {
        const totalCost = recentExecutions.reduce((sum: number, e: { costUsd: number }) => sum + e.costUsd, 0);
        if (totalCost > config.threshold) {
          triggered = true;
          message = `Total cost $${totalCost.toFixed(4)} exceeds threshold $${config.threshold}`;
        }
        break;
      }
    }

    if (triggered) {
      const alert = this.alertRepo.createAlert({
        ruleId: rule.id,
        ruleName: rule.name,
        severity: 'warning',
        message,
      });
      this.sseHub.broadcast({
        type: 'alert',
        data: { alertRuleId: rule.id, message, alertId: alert.id },
        timestamp: new Date().toISOString(),
      });
    }
  }
}
