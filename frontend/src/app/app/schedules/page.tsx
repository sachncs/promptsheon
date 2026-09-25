'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CalendarClock, Plus, Trash2 } from 'lucide-react';
import { getErrorMessage, scheduleApi, workspaceApi, releaseApi, type Release, type Schedule } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { QueryError } from '@/components/brand/query-error';

const KIND_OPTIONS = [
  { value: 'eval', label: 'Eval run' },
  { value: 'self-evolve', label: 'Self-evolve cycle' },
  { value: 'release-rotation', label: 'Release rotation' },
] as const;

type ScheduleKind = typeof KIND_OPTIONS[number]['value'];

function isScheduleKind(value: string): value is ScheduleKind {
  return KIND_OPTIONS.some((option) => option.value === value);
}

export default function SchedulesPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = workspaces.data?.[0];
  const wsId = wsFirst?.id;

  const allReleases = useQuery({
    queryKey: ['releases-for-schedules'],
    queryFn: () => releaseApi.listAll(1, 100).then((res) => res.data.items),
    enabled: Boolean(wsId),
  });

  const schedules = useQuery({
    queryKey: ['schedules'],
    queryFn: () => scheduleApi.list().then((r) => r.data),
  });
  const rows: Schedule[] = schedules.data?.items ?? [];

  const [releaseId, setReleaseId] = useState('');
  const [kind, setKind] = useState<ScheduleKind>('eval');
  const [cron, setCron] = useState('0 */6 * * *');

  const create = useMutation({
    mutationFn: () => {
      if (!wsId) throw new Error('No active workspace');
      return scheduleApi.create({ workspaceId: wsId, releaseId, kind, cron });
    },
    onSuccess: () => {
      setReleaseId('');
      void qc.invalidateQueries({ queryKey: ['schedules'] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => scheduleApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules'] }),
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={getErrorMessage(workspaces.error)} onRetry={() => void workspaces.refetch()} />;
  if (allReleases.isError) return <QueryError message={getErrorMessage(allReleases.error)} onRetry={() => void allReleases.refetch()} />;
  if (schedules.isError) return <QueryError message={getErrorMessage(schedules.error)} onRetry={() => void schedules.refetch()} />;
  if (workspaces.isPending || (Boolean(wsId) && allReleases.isPending) || schedules.isPending) {
    return (
      <div className="space-y-6" aria-busy="true">
        <PageHeader eyebrow="Release" title="Schedules" subtitle="Cron-based schedules for eval runs, release rotations, and self-evolve cycles." />
        <Surface className="h-72 animate-pulse bg-surface-2/40"><span className="sr-only">Loading schedules</span></Surface>
      </div>
    );
  }

  const releases: Release[] = allReleases.data ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Schedules"
        subtitle="Cron-based schedules for eval runs, release rotations, and self-evolve cycles."
      />

      <Surface>
        <SurfaceHeader
          title="New schedule"
          description={wsId ? `In workspace ${wsFirst?.name ?? wsId.slice(0, 8)}` : 'Create a workspace before scheduling a release.'}
        />
        <div className="grid gap-3 sm:grid-cols-4">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Release</label>
            <div className="mt-2">
              <ThemedSelect
                value={releaseId || undefined}
                onValueChange={setReleaseId}
                placeholder="— pick a release —"
                options={releases.map((r) => ({
                  value: r.id,
                  label: `${r.capabilityId} v${r.capabilityVersion} · ${r.environment}`,
                }))}
                ariaLabel="Pick a release"
                triggerClassName="w-full"
              />
            </div>
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Kind</label>
            <div className="mt-2">
              <ThemedSelect
                value={kind}
                onValueChange={(v) => { if (isScheduleKind(v)) setKind(v); }}
                options={KIND_OPTIONS.map((k) => ({ value: k.value, label: k.label }))}
                ariaLabel="Schedule kind"
                triggerClassName="w-full"
              />
            </div>
          </div>
          <div>
            <label htmlFor="schedule-cron" className="text-xs uppercase tracking-wider text-text-subtle">Cron</label>
            <Input
              id="schedule-cron"
              value={cron}
              onChange={(e) => setCron(e.target.value)}
              placeholder="0 */6 * * *"
              className="mt-2 font-mono"
            />
          </div>
          <div className="flex items-end">
            <Button
              onClick={() => create.mutate()}
              disabled={!wsId || !releaseId || !cron.trim() || create.isPending}
              className="w-full"
            >
              <Plus className="mr-1.5 size-3.5" />
              {create.isPending ? 'Scheduling…' : 'Schedule'}
            </Button>
          </div>
        </div>
        {create.isError && <div role="alert" className="mt-3 text-xs text-destructive">{getErrorMessage(create.error, 'The schedule could not be created.')}</div>}
        {!allReleases.isPending && releases.length === 0 && wsId && (
          <p className="mt-3 text-xs text-text-muted">Publish a release first; schedules can only target published releases.</p>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Active schedules"
          description={`${rows.length} configured`}
        />
        {rows.length === 0 ? (
          <EmptyState
            icon={CalendarClock}
            title="No schedules yet"
            description="Create a schedule to fire on a cron — nightly eval runs, weekly rotations, or self-evolve cycles."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => r.id}
            columns={[
              {
                key: 'kind',
                header: 'Kind',
                render: (r) => <Badge>{r.kind}</Badge>,
              },
              {
                key: 'release',
                header: 'Release',
                render: (r) => (
                  <span className="font-mono text-xs text-text-muted">{r.releaseId.slice(0, 16)}…</span>
                ),
              },
              {
                key: 'cron',
                header: 'Cron',
                render: (r) => <code className="font-mono text-xs">{r.cron}</code>,
              },
              {
                key: 'last',
                header: 'Last run',
                render: (r) => r.lastFireAt ? new Date(r.lastFireAt).toLocaleString() : '—',
              },
              {
                key: 'next',
                header: 'Next run',
                render: (r) => new Date(r.nextFireAt).toLocaleString(),
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      if (window.confirm('Delete this schedule? This cannot be undone.')) remove.mutate(r.id);
                    }}
                    disabled={remove.isPending}
                  >
                    <Trash2 className="mr-1 size-3" />
                    {remove.isPending ? 'Deleting…' : 'Delete'}
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
