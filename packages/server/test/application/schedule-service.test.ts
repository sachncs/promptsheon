import { describe, expect, it, vi } from 'vitest';
import type { Schedule } from '@promptsheon/shared';
import { InvalidScheduleCronError, ScheduleService, type ScheduleStore } from '../../src/application/schedule-service.js';

const schedule: Schedule = {
  id: 'schedule-1',
  workspaceId: 'workspace-1',
  releaseId: 'release-1',
  kind: 'eval',
  cron: '* * * * *',
  webhookPath: '',
  nextFireAt: '2026-01-01T00:01:00.000Z',
  lastFireAt: null,
  firedCount: 0,
  enabled: true,
  createdAt: '2026-01-01T00:00:00.000Z',
  createdBy: 'user-1',
};

function store(): ScheduleStore {
  return {
    findMany: vi.fn(() => ({ items: [schedule], total: 1 })),
    findById: vi.fn(() => schedule),
    create: vi.fn(() => schedule),
    update: vi.fn(() => schedule),
    delete: vi.fn(() => true),
  };
}

describe('ScheduleService', () => {
  it('validates cron expressions before creating a schedule', () => {
    const schedules = store();
    const service = new ScheduleService(schedules);

    expect(() => service.create({ workspaceId: 'w', releaseId: 'r', kind: 'eval', cron: 'invalid' }))
      .toThrow(InvalidScheduleCronError);
    expect(schedules.create).not.toHaveBeenCalled();
  });

  it('validates changed cron expressions before updating a schedule', () => {
    const schedules = store();
    const service = new ScheduleService(schedules);

    expect(() => service.update('schedule-1', { cron: 'invalid' })).toThrow(InvalidScheduleCronError);
    expect(schedules.update).not.toHaveBeenCalled();
  });

  it('delegates valid schedule operations to the store', () => {
    const schedules = store();
    const service = new ScheduleService(schedules);

    expect(service.list({ page: 1, pageSize: 10 })).toEqual({ items: [schedule], total: 1 });
    expect(service.get('schedule-1')).toEqual(schedule);
    expect(service.create({ workspaceId: 'w', releaseId: 'r', kind: 'eval', cron: '* * * * *' })).toEqual(schedule);
    expect(service.update('schedule-1', { enabled: false })).toEqual(schedule);
    expect(service.delete('schedule-1')).toBe(true);
    expect(schedules.create).toHaveBeenCalledOnce();
    expect(schedules.update).toHaveBeenCalledWith('schedule-1', { enabled: false });
    expect(schedules.delete).toHaveBeenCalledWith('schedule-1');
  });
});
