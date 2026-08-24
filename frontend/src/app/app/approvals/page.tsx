'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ShieldCheck, ShieldAlert, Inbox } from 'lucide-react';
import { workspaceApi, projectApi, releaseApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Button } from '@/components/ui/button';

interface Release {
  id: string;
  capabilityId?: string;
  capabilityName?: string;
  capabilityVersion?: number;
  environment?: string;
  state?: string;
  canaryPercent?: number;
  manifestHash?: string;
  updatedAt?: string;
  approvals?: Array<{ voter: string; decision: 'approve' | 'reject'; at: string }>;
}

export default function ApprovalsPage() {
  const session = useRequireSession();
  const router = useRouter();

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
    queryKey: ['approvals', 'releases', projectList.map((p) => p.id)],
    queryFn: async (): Promise<Release[]> => {
      const out: Release[] = [];
      for (const p of projectList) {
        try {
          const r = await releaseApi.list(p.id).then((res) => res.data);
          if (Array.isArray(r)) {
            for (const rel of r) {
              const base = rel as Release;
              const merged: Release = { ...base, capabilityId: base.capabilityId ?? p.id };
              if (p.name !== undefined) merged.capabilityName = p.name;
              out.push(merged);
            }
          }
        } catch { /* skip */ }
      }
      return out;
    },
    enabled: projectList.length > 0,
  });

  if (!session) return null;

  const rows = ((allReleases.data ?? []) as Release[]).filter(
    (r) => r.state === 'review' || r.state === 'draft',
  );

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Quality"
        title="Approvals queue"
        subtitle="Releases waiting for a second pair of eyes. Maker-checker enforcement: the creator cannot approve their own release."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Pending review"
          description={`${rows.length} release(s) need attention`}
        />
        {rows.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title="No approvals queued"
            description="When a release enters review or draft state, it appears here for a second reviewer."
            action={
              <Link href="/app/releases">
                <Button>Browse releases</Button>
              </Link>
            }
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { router.push(`/app/releases/${String(r['id'])}`); }}
            columns={[
              {
                key: 'cap',
                header: 'Capability',
                render: (r) => (
                  <div>
                    <div className="font-medium text-text-strong">{String(r['capabilityName'] ?? '—')}</div>
                    <div className="text-xs text-text-subtle">v{String(r['capabilityVersion'] ?? '?')} · {String(r['environment'] ?? '—')}</div>
                  </div>
                ),
              },
              {
                key: 'approvals',
                header: 'Votes',
                render: (r) => {
                  const a = (r['approvals'] as Release['approvals']) ?? [];
                  if (a.length === 0) return <span className="text-text-subtle text-xs">no votes yet</span>;
                  return (
                    <div className="flex items-center gap-1.5">
                      {a.map((v, i) => (
                        <span
                          key={i}
                          className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                            v.decision === 'approve'
                              ? 'bg-success/15 text-success'
                              : 'bg-destructive/15 text-destructive'
                          }`}
                          title={`${v.voter} · ${new Date(v.at).toLocaleString()}`}
                        >
                          {v.decision === 'approve' ? <ShieldCheck className="size-3" /> : <ShieldAlert className="size-3" />}
                          {v.voter}
                        </span>
                      ))}
                    </div>
                  );
                },
              },
              {
                key: 'canary',
                header: 'Canary',
                render: (r) => {
                  const pct = Number(r['canaryPercent'] ?? 0);
                  return <span className="font-mono text-xs text-text-muted">{pct}%</span>;
                },
              },
              {
                key: 'hash',
                header: 'Hash',
                render: (r) => r['manifestHash'] ? <HashChip hash={String(r['manifestHash'])} /> : <span className="text-text-muted">—</span>,
              },
              {
                key: 'state',
                header: 'State',
                render: (r) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} />,
              },
              {
                key: 'when',
                header: 'Updated',
                render: (r) => r['updatedAt'] ? new Date(String(r['updatedAt'])).toLocaleString() : '—',
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}