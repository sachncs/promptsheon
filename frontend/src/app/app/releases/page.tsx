'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { GitBranch, GitMerge, Plus, ArrowUpRight } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, projectApi, capabilityApi, releaseApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Tabs, TabsList, TabsTrigger } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NewReleaseDialog } from '@/components/brand/new-release-dialog';

type FilterState = 'all' | 'draft' | 'review' | 'approved' | 'canary' | 'active' | 'rolled-back';

export default function ReleasesPage() {
  const session = useRequireSession();
  const router = useRouter();
  const [filter, setFilter] = useState<FilterState>('all');
  const [search, setSearch] = useState('');

  const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1).then((r) => r.data) });
  const wsFirst = unwrapFirst<{ id: string }>(workspaces.data);

  const projects = useQuery({
    queryKey: ['projects', wsFirst?.id],
    queryFn: () => projectApi.list(wsFirst!.id).then((r) => r.data),
    enabled: Boolean(wsFirst?.id),
  });
  const allProjects = unwrapArray<{ id: string }>(projects.data);

  const capabilities = useQuery({
    queryKey: ['capabilities', 'all', allProjects.map((p) => p.id)],
    queryFn: async () => {
      const out: Array<Record<string, unknown>> = [];
      for (const p of allProjects) {
        const list = await capabilityApi.list(p.id).then((r) => r.data).catch(() => []);
        if (Array.isArray(list)) out.push(...(list as Array<Record<string, unknown>>));
      }
      return out;
    },
    enabled: allProjects.length > 0,
  });

  const releases = useQuery({
    queryKey: ['releases', 'all', capabilities.data],
    queryFn: async () => {
      const out: Array<Record<string, unknown>> = [];
      const caps = capabilities.data ?? [];
      for (const c of caps) {
        const list = await releaseApi.list(String(c['id'])).then((r) => r.data).catch(() => []);
        if (Array.isArray(list)) {
          out.push(...list.map((rel: Record<string, unknown>) => ({
            ...rel,
            capabilityName: String(c['name'] ?? '—'),
            capabilityId: String(c['id']),
          })));
        }
      }
      return out.sort((a, b) => String(b['createdAt'] ?? '').localeCompare(String(a['createdAt'] ?? '')));
    },
    enabled: Array.isArray(capabilities.data) && capabilities.data.length > 0,
  });

  const rows = useMemo(() => {
    const arr = Array.isArray(releases.data) ? releases.data : [];
    return arr.filter((r) => {
      if (filter !== 'all' && String(r['state'] ?? '') !== filter) return false;
      if (search) {
        const q = search.toLowerCase();
        if (!String(r['capabilityName'] ?? '').toLowerCase().includes(q) &&
            !String(r['manifestHash'] ?? '').toLowerCase().includes(q)) return false;
      }
      return true;
    });
  }, [releases.data, filter, search]);

  if (!session) return null;

  const total = Array.isArray(releases.data) ? releases.data.length : 0;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Releases"
        subtitle="Every promotion of a capability is a first-class artifact. Draft → review → approved → canary → active → rolled back."
        actions={<NewReleaseDialog />}
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs value={filter} onValueChange={(v) => setFilter(v as FilterState)}>
          <TabsList>
            <TabsTrigger value="all">All ({total})</TabsTrigger>
            <TabsTrigger value="draft">Draft</TabsTrigger>
            <TabsTrigger value="review">Review</TabsTrigger>
            <TabsTrigger value="approved">Approved</TabsTrigger>
            <TabsTrigger value="canary">Canary</TabsTrigger>
            <TabsTrigger value="active">Active</TabsTrigger>
            <TabsTrigger value="rolled-back">Rolled back</TabsTrigger>
          </TabsList>
        </Tabs>
        <Input
          placeholder="Filter by capability or hash…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={GitBranch}
          title={total === 0 ? 'No releases yet' : 'No releases match the filter'}
          description={total === 0
            ? 'Author a capability and create its first release. Releases inherit the deterministic state machine.'
            : 'Try clearing the filter or the search input.'}
          action={
            total === 0 ? (
              <Link href="/app/editor"><Button variant="outline">Open the DAG editor</Button></Link>
            ) : (
              <Button variant="outline" onClick={() => { setFilter('all'); setSearch(''); }}>Clear filter</Button>
            )
          }
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {rows.map((r) => {
            const id = String(r['id']);
            const name = String(r['capabilityName'] ?? '—');
            const version = String(r['capabilityVersion'] ?? '?');
            const env = String(r['environment'] ?? 'production');
            const state = String(r['state'] ?? 'neutral');
            const canary = Number(r['canaryPercent'] ?? 0);
            const hash = String(r['manifestHash'] ?? id);
            const updated = new Date(String(r['updatedAt'] ?? r['createdAt'] ?? Date.now()));
            return (
              <button
                key={id}
                type="button"
                onClick={() => router.push(`/app/releases/${id}`)}
                className="group rounded-2xl border border-border-subtle bg-surface-1 p-5 text-left shadow-1 transition-all hover:border-border-strong hover:shadow-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-text-subtle">v{version} · {env}</div>
                    <div className="mt-1 truncate font-semibold text-text-strong">{name}</div>
                  </div>
                  <ArrowUpRight className="size-4 shrink-0 text-text-subtle opacity-0 transition-opacity group-hover:opacity-100" />
                </div>
                <div className="mt-3 flex items-center gap-2">
                  <StatusPill kind={(state as never) ?? 'neutral'} />
                  {canary > 0 && (
                    <div className="flex items-center gap-2">
                      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-surface-2">
                        <div className="h-full bg-brand" style={{ width: `${canary}%` }} />
                      </div>
                      <span className="font-mono text-xs text-text-muted">{canary}%</span>
                    </div>
                  )}
                </div>
                <div className="mt-4 flex items-center justify-between">
                  <HashChip hash={hash} />
                  <span className="text-xs text-text-subtle">{updated.toLocaleDateString()}</span>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function unwrapArray<T = unknown>(data: unknown): T[] {
  if (Array.isArray(data)) return data as T[];
  if (data && typeof data === 'object' && 'items' in data) {
    const items = (data as { items?: unknown }).items;
    if (Array.isArray(items)) return items as T[];
  }
  return [];
}

function unwrapFirst<T = unknown>(data: unknown): T | undefined {
  return unwrapArray<T>(data)[0];
}