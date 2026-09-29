import type { CanaryRollbackService } from '../application/canary-rollback-service.js';
import type { ReleaseRepo } from '../repos/release.js';

export interface CanaryRollbackMonitorLogger {
  info(data: Record<string, unknown>, message: string): void;
  error(data: Record<string, unknown>, message: string): void;
}

/** Periodically assesses every canary and stops cleanly during shutdown. */
export class CanaryRollbackMonitor {
  private interval: ReturnType<typeof setInterval> | null = null;
  private running = false;

  constructor(
    private readonly releases: ReleaseRepo,
    private readonly rollback: CanaryRollbackService,
    private readonly logger: CanaryRollbackMonitorLogger,
  ) {}

  start(periodMs = 60_000): void {
    if (this.interval) return;
    this.interval = setInterval(() => { void this.poll(); }, periodMs);
    void this.poll();
  }

  stop(): void {
    if (this.interval) clearInterval(this.interval);
    this.interval = null;
  }

  async poll(): Promise<void> {
    if (this.running) return;
    this.running = true;
    try {
      for (const candidate of this.releases.listCanaryReleases()) {
        const result = this.rollback.assess(candidate.releaseId, candidate.organizationId, 'system:canary-monitor');
        if (result.action === 'rolled_back') {
          this.logger.info({ ...result }, 'canary release automatically rolled back');
        }
      }
    } catch (error) {
      this.logger.error({ error }, 'canary rollback monitor poll failed');
    } finally {
      this.running = false;
    }
  }
}
