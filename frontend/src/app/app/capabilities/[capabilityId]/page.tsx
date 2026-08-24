'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import {
  ArrowLeft, Workflow, FlaskConical, GitBranch, ScrollText, ShieldCheck,
  Box, Boxes,
} from 'lucide-react';
import { capabilityApi, versionApi, manifestApi, releaseApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Timeline } from '@/components/brand/timeline';
import { DataTable } from '@/components/brand/data-table';
import { DagMini } from '@/components/brand/dag-mini';
import { EmptyState } from '@/components/brand/empty-state';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';

type Tab = 'overview' | 'versions' | 'graph' | 'releases';

export default function CapabilityDetailPage() {
  const params = useParams<{ capabilityId: string }>();
  const id = params.capabilityId;
  const router = useRouter();
  const [tab, setTab] = useState<Tab>('overview');

  const cap = useQuery({
    queryKey: ['capability', id],
    queryFn: () => capabilityApi.get(id).then((r) => r.data),
    enabled: Boolean(id),
  });

  const versions = useQuery({
    queryKey: ['versions', id],
    queryFn: () => versionApi.list(id).then((r) => r.data).catch(() => []),
    enabled: Boolean(id),
  });

  const versionList = Array.isArray(versions.data) ? versions.data : [];

  const manifest = useQuery({
    queryKey: ['manifest', id],
    queryFn: () => manifestApi.get(id).then((r) => r.data).catch(() => null),
    enabled: Boolean(id),
  });

  const releases = useQuery({
    queryKey: ['releases', id],
    queryFn: () => releaseApi.list(id).then((r) => r.data).catch(() => []),
    enabled: Boolean(id),
  });
  const releaseList = Array.isArray(releases.data) ? releases.data : [];

  if (cap.isLoading) {
    return <div className="text-text-muted text-sm">Loading…</div>;
  }

  if (!cap.data) {
    return (
      <EmptyState
        icon={Boxes}
        title="Capability not found"
        description="This capability may have been deleted, or the link is stale."
        action={<Link href="/app/capabilities"><Button variant="outline"><ArrowLeft className="mr-1.5 h-3.5 w-3.5" />Back to registry</Button></Link>}
      />
    );
  }

  const c = cap.data as { name?: string; description?: string; id: string; manifestHash?: string; state?: string };

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/capabilities" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Registry
        </Link>
        <PageHeader
          eyebrow="Capability"
          title={c.name ?? 'Untitled capability'}
          subtitle={c.description ?? 'A multi-agent DAG composed of prompts, policies, tools, and guardrails.'}
          actions={
            <div className="flex items-center gap-2">
              {c.manifestHash && <HashChip hash={c.manifestHash} />}
              <StatusPill kind={(c.state as never) ?? 'active'} />
              <Link href={`/app/diff?capability=${c.id}`}>
                <Button variant="outline" size="sm">Diff a version</Button>
              </Link>
            </div>
          }
        />
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="versions">Versions</TabsTrigger>
          <TabsTrigger value="graph">Graph</TabsTrigger>
          <TabsTrigger value="releases">Releases</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <div className="grid gap-5 lg:grid-cols-2">
            <Surface>
              <SurfaceHeader title="Identity" />
              <dl className="space-y-3 text-sm">
                <Detail label="Identifier" value={c.id} mono />
                <Detail label="Name" value={c.name ?? '—'} />
                <Detail
                  label="Manifest hash"
                  value={<HashChip hash={c.manifestHash ?? '—'} />}
                />
                <Detail label="Versions" value={`${versionList.length}`} />
                <Detail label="Open releases" value={`${releaseList.length}`} />
              </dl>
            </Surface>

            <Surface>
              <SurfaceHeader title="Recent versions" description="Append-only history of immutable artifacts." />
              {versionList.length === 0 ? (
                <EmptyState icon={Box} title="No versions yet" description="Compile a draft to create the first version." />
              ) : (
                <Timeline
                  entries={versionList.slice(0, 6).map((v: Record<string, unknown>) => ({
                    id: String(v['id']),
                    title: `v${String(v['version'] ?? '?')}`,
                    description: String(v['summary'] ?? 'Compiled'),
                    timestamp: new Date(String(v['createdAt'] ?? Date.now())).toLocaleString(),
                    icon: GitBranch,
                    tone: 'info',
                  }))}
                />
              )}
            </Surface>
          </div>
        </TabsContent>

        <TabsContent value="versions">
          <Surface padded={false}>
            <SurfaceHeader className="px-5 pt-5" title={`${versionList.length} versions`} />
            <DataTable
              className="rounded-none border-0 border-t border-border-subtle"
              rows={versionList}
              rowKey={(r: Record<string, unknown>) => String(r['id'])}
              columns={[
                { key: 'v', header: 'Version', render: (r: Record<string, unknown>) => <span className="font-mono text-xs">v{String(r['version'] ?? '?')}</span> },
                { key: 'hash', header: 'Hash', render: (r: Record<string, unknown>) => <HashChip hash={String(r['manifestHash'] ?? r['id'])} /> },
                { key: 'author', header: 'Author', render: (r: Record<string, unknown>) => String(r['createdBy'] ?? 'system') },
                { key: 'created', header: 'Created', render: (r: Record<string, unknown>) => new Date(String(r['createdAt'] ?? Date.now())).toLocaleString() },
                {
                  key: 'actions',
                  header: '',
                  render: (r: Record<string, unknown>) => (
                    <Link href={`/app/diff?capability=${id}&version=${String(r['version'] ?? '')}`} className="text-xs text-brand-highlight hover:underline">
                      Diff
                    </Link>
                  ),
                },
              ]}
            />
          </Surface>
        </TabsContent>

        <TabsContent value="graph">
          <Surface>
            <SurfaceHeader title="Multi-agent DAG" description="The structure of this capability: agents, tools, memory, policies, and the edges between them." />
            <DagMini
              nodes={nodesForManifest(manifest.data as Record<string, unknown> | null)}
              edges={edgesForManifest(manifest.data as Record<string, unknown> | null)}
              className="mt-3 rounded-lg border border-border-subtle bg-surface-0"
            />
          </Surface>
        </TabsContent>

        <TabsContent value="releases">
          <Surface padded={false}>
            <SurfaceHeader className="px-5 pt-5" title={`${releaseList.length} releases`} />
            <DataTable
              className="rounded-none border-0 border-t border-border-subtle"
              rows={releaseList}
              rowKey={(r: Record<string, unknown>) => String(r['id'])}
              onRowClick={(r) => { router.push(`/app/releases/${String(r['id'])}`); }}
              columns={[
                { key: 'v', header: 'Version', render: (r: Record<string, unknown>) => `v${String(r['capabilityVersion'] ?? '?')}` },
                { key: 'env', header: 'Environment', render: (r: Record<string, unknown>) => <span className="font-mono text-xs">{String(r['environment'] ?? 'production')}</span> },
                { key: 'state', header: 'State', render: (r: Record<string, unknown>) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} /> },
                { key: 'hash', header: 'Content', render: (r: Record<string, unknown>) => <HashChip hash={String(r['manifestHash'] ?? r['id'])} /> },
                { key: 'canary', header: 'Canary', render: (r: Record<string, unknown>) => r['canaryPercent'] != null ? `${String(r['canaryPercent'])}%` : '—' },
              ]}
            />
          </Surface>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function Detail({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-xs uppercase tracking-wider text-text-subtle">{label}</dt>
      <dd className={mono ? 'font-mono text-xs text-text-default' : 'text-sm text-text-default'}>{value}</dd>
    </div>
  );
}

interface ManifestLike { nodes?: Array<{ id: string; label?: string }>; edges?: Array<{ from: string; to: string }> }

function nodesForManifest(m: Record<string, unknown> | null | undefined): Array<{ id: string; label: string }> {
  const ml = (m ?? {}) as ManifestLike;
  if (Array.isArray(ml.nodes)) {
    return ml.nodes.map((n) => ({ id: n.id, label: n.label ?? n.id }));
  }
  return [
    { id: 'classify', label: 'Classify' },
    { id: 'retrieve', label: 'Retrieve' },
    { id: 'decide', label: 'Decide' },
    { id: 'respond', label: 'Respond' },
  ];
}

function edgesForManifest(m: Record<string, unknown> | null | undefined): Array<{ from: string; to: string }> {
  const ml = (m ?? {}) as ManifestLike;
  if (Array.isArray(ml.edges)) return ml.edges;
  return [
    { from: 'classify', to: 'retrieve' },
    { from: 'retrieve', to: 'decide' },
    { from: 'decide', to: 'respond' },
  ];
}
