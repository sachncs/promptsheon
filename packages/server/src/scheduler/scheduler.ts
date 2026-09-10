import type { Schedule } from '@promptsheon/shared';
import type { ScheduleRepo } from '../repos/schedule.js';
import type { SseHub } from '../sse/hub.js';

export type ScheduleHandler = (schedule: Schedule) => Promise<void>;

export class Scheduler {
  private interval: ReturnType<typeof setInterval> | null = null;
  private running = false;
  private handlers = new Map<string, ScheduleHandler>();

  constructor(
    private scheduleRepo: ScheduleRepo,
    private sseHub: SseHub,
  ) {}

  registerHandler(kind: string, handler: ScheduleHandler): void {
    this.handlers.set(kind, handler);
  }

  start(pollIntervalMs = 10_000): void {
    this.interval = setInterval(() => { this.poll().catch(console.error); }, pollIntervalMs);
    this.poll().catch(console.error);
  }

  stop(): void {
    if (this.interval) clearInterval(this.interval);
  }

  async poll(): Promise<void> {
    if (this.running) return;
    this.running = true;

    try {
      const now = new Date();
      const dueSchedules = await this.scheduleRepo.findDueSchedules(now);

      for (const schedule of dueSchedules) {
        const handler = this.handlers.get(schedule.kind);
        if (!handler) continue;

        try {
          await handler(schedule);
          await this.scheduleRepo.update(schedule.id, { nextFireAt: new Date().toISOString() });
          this.sseHub.broadcast({
            type: 'status',
            data: { scheduleId: schedule.id, status: 'fired' },
            timestamp: new Date().toISOString(),
          });
        } catch (e) {
          console.error(`Schedule ${schedule.id} failed:`, e);
          this.sseHub.broadcast({
            type: 'error',
            data: { scheduleId: schedule.id, error: String(e) },
            timestamp: new Date().toISOString(),
          });
        }
      }
    } finally {
      this.running = false;
    }
  }
}
