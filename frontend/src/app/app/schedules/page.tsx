'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CalendarClock, Plus, Trash2 } from 'lucide-react';
import { scheduleApi, workspaceApi, releaseApi, projectApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';

interface ScheduleItem {
  id: string;
  workspaceId?: string;
  releaseId?: string;
  releaseName?: string;
  kind?: string;
  cron?: string;
  enabled?: boolean;
  createdAt?: string;
  lastRunAt?: string | null;
  nextRunAt?: string | null;
}

interface ReleaseLite {
  id: string;
  capabilityName?: string;
  capabilityVersion?: number;
  environment?: string;
}

const KIND_OPTIONS = [
  { value: 'eval', label: 'Eval run' },
  { value: 'self-evolve', label: 'Self-evolve cycle' },
  { value: 'release-rotation', label: 'Release rotation' },
] as const;

export default function SchedulesPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] as { id?: string } : undefined;
  const wsId = wsFirst?.id;

  const projects = useQuery({
    queryKey: ['projects', wsId],
    queryFn: () => (wsId ? projectApi.list(wsId).then((r) => r.data) : Promise.resolve([])),
    enabled: Boolean(wsId),
  });
  const projectList = Array.isArray(projects.data) ? projects.data as Array<{ id: string; name?: string }> : [];

  const allReleases = useQuery({
    queryKey: ['releases-for-schedules', projectList.map((p) => p.id)],
    queryFn: async (): Promise<ReleaseLite[]> => {
      const out: ReleaseLite[] = [];
      for (const p of projectList) {
        try {
          const r = await releaseApi.list(p.id).then((res) => res.data);
          if (Array.isArray(r)) {
            for (const rel of r) {
              const lite: ReleaseLite = { id: String((rel as { id?: unknown }).id ?? '') };
              if (p.name !== undefined) lite.capabilityName = p.name;
              const cv = Number((rel as { capabilityVersion?: unknown }).capabilityVersion ?? 0);
              if (cv) lite.capabilityVersion = cv;
              const env = (rel as { environment?: unknown }).environment as string | undefined;
              if (env !== undefined) lite.environment = env;
              out.push(lite);
            }
          }
        } catch { /* skip */ }
      }
      return out.filter((r) => r.id);
    },
    enabled: projectList.length > 0,
  });

  const schedules = useQuery({
    queryKey: ['schedules'],
    queryFn: () => scheduleApi.list().then((r) => r.data).catch(() => [] as ScheduleItem[]),
  });
  const rows = (schedules.data ?? []) as ScheduleItem[];

  const [releaseId, setReleaseId] = useState('');
  const [kind, setKind] = useState<typeof KIND_OPTIONS[number]['value']>('eval');
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

  const releases = (allReleases.data ?? []) as ReleaseLite[];

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
          description={wsId ? `In workspace ${wsFirst?.id?.slice(0, 8) ?? ''}` : 'No workspace available'}
        />
        <div className="grid gap-3 sm:grid-cols-4">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Release</label>
            <select
              value={releaseId}
              onChange={(e) => setReleaseId(e.target.value)}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
            >
              <option value="">— pick a release —</option>
              {releases.map((r) => (
                <option key={r.id} value={r.id}>
                  {(r.capabilityName ?? '?') + ' v' + (r.capabilityVersion ?? '?') + ' · ' + (r.environment ?? '—')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Kind</label>
            <select
              value={kind}
              onChange={(e) => setKind(e.target.value as typeof KIND_OPTIONS[number]['value'])}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
            >
              {KIND_OPTIONS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
            </select>
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Cron</label>
            <Input
              value={cron}
              onChange={(e) => setCron(e.target.value)}
              placeholder="0 */6 * * *"
              className="mt-2 font-mono"
            />
          </div>
          <div className="flex items-end">
            <Button
              onClick={() => create.mutate()}
              disabled={!releaseId || !cron || create.isPending}
              className="w-full"
            >
              <Plus className="mr-1.5 size-3.5" />
              {create.isPending ? 'Scheduling…' : 'Schedule'}
            </Button>
          </div>
        </div>
        {create.isError && (
          <div className="mt-3 text-xs text-destructive">{(create.error as Error).message}</div>
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
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              {
                key: 'kind',
                header: 'Kind',
                render: (r) => <Badge>{String(r['kind'] ?? 'eval')}</Badge>,
              },
              {
                key: 'release',
                header: 'Release',
                render: (r) => (
                  <span className="font-mono text-xs text-text-muted">{String(r['releaseId'] ?? '—').slice(0, 16)}…</span>
                ),
              },
              {
                key: 'cron',
                header: 'Cron',
                render: (r) => <code className="font-mono text-xs">{String(r['cron'] ?? '—')}</code>,
              },
              {
                key: 'last',
                header: 'Last run',
                render: (r) => r['lastRunAt'] ? new Date(String(r['lastRunAt'])).toLocaleString() : '—',
              },
              {
                key: 'next',
                header: 'Next run',
                render: (r) => r['nextRunAt'] ? new Date(String(r['nextRunAt'])).toLocaleString() : '—',
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => remove.mutate(String(r['id']))}
                  >
                    <Trash2 className="mr-1 size-3" />
                    Delete
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