import type { Schedule } from '@promptsheon/shared';
import type { Paginated } from '../repos/base.js';
import { nextCronFire } from '../scheduler/cron.js';

/** Persistence operations required by the schedule use case. */
export interface ScheduleStore {
  findMany(opts: { page: number; pageSize: number }): Paginated<Schedule>;
  findById(id: string): Schedule | null;
  create(data: { workspaceId: string; releaseId: string; kind: string; cron: string; enabled?: boolean }): Schedule;
  update(id: string, data: Partial<Pick<Schedule, 'cron' | 'enabled' | 'nextFireAt'>>): Schedule | null;
  delete(id: string): boolean;
}

/** Error raised when a schedule cron expression cannot be evaluated. */
export class InvalidScheduleCronError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'InvalidScheduleCronError';
  }
}

/** Application use case for validating and persisting schedules. */
export class ScheduleService {
  constructor(
    private readonly store: ScheduleStore,
    private readonly clock: () => Date = () => new Date(),
  ) {}

  list(opts: { page: number; pageSize: number }): Paginated<Schedule> {
    return this.store.findMany(opts);
  }

  get(id: string): Schedule | null {
    return this.store.findById(id);
  }

  create(data: { workspaceId: string; releaseId: string; kind: string; cron: string; enabled?: boolean }): Schedule {
    this.validateCron(data.cron);
    return this.store.create(data);
  }

  update(id: string, data: Partial<Pick<Schedule, 'cron' | 'enabled' | 'nextFireAt'>>): Schedule | null {
    if (data.cron) this.validateCron(data.cron);
    return this.store.update(id, data);
  }

  delete(id: string): boolean {
    return this.store.delete(id);
  }

  private validateCron(cron: string): void {
    try {
      nextCronFire(cron, this.clock());
    } catch (error) {
      throw new InvalidScheduleCronError(error instanceof Error ? error.message : 'Invalid cron expression');
    }
  }
}
