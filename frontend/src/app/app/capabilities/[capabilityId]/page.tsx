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
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Timeline } from '@/components/brand/timeline';
import { DataTable } from '@/components/brand/data-table';
import { DagMini } from '@/components/brand/dag-mini';
import { EmptyState } from '@/components/brand/empty-state';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';
import { QueryError } from '@/components/brand/query-error';

type Tab = 'overview' | 'versions' | 'graph' | 'releases';

export default function CapabilityDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ capabilityId: string }>();
  const id = params.capabilityId;
  const router = useRouter();
  const [tab, setTab] = useState<Tab>('overview');

  const cap = useQuery({
    queryKey: ['capability', id],
    queryFn: () => capabilityApi.get(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });

  const versions = useQuery({
    queryKey: ['versions', id],
    queryFn: () => versionApi.list(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });

  const versionList = Array.isArray(versions.data) ? versions.data : [];

  const manifest = useQuery({
    queryKey: ['manifest', id],
    queryFn: () => manifestApi.get(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });

  const releases = useQuery({
    queryKey: ['releases', id],
    queryFn: () => releaseApi.list(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });
  const releaseList = Array.isArray(releases.data) ? releases.data : [];

  if (cap.isLoading) {
    return <div className="text-text-muted text-sm">Loading…</div>;
  }

  if (cap.isError) return <QueryError message={cap.error} onRetry={() => void cap.refetch()} />;
  if (versions.isError) return <QueryError message={versions.error} onRetry={() => void versions.refetch()} />;
  if (manifest.isError) return <QueryError message={manifest.error} onRetry={() => void manifest.refetch()} />;
  if (releases.isError) return <QueryError message={releases.error} onRetry={() => void releases.refetch()} />;

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
              <StatusPill kind={statusKindOf(c.state, 'active')} />
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
                  entries={versionList.slice(0, 6).map((v) => ({
                    id: v.id,
                    title: `v${v.version}`,
                    description: 'Compiled',
                    timestamp: new Date(v.createdAt).toLocaleString(),
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
              rowKey={(r) => r.id}
              columns={[
                { key: 'v', header: 'Version', render: (r) => <span className="font-mono text-xs">v{r.version}</span> },
                { key: 'hash', header: 'Hash', render: (r) => <HashChip hash={r.manifestHash || r.id} /> },
                { key: 'author', header: 'Author', render: (r) => r.createdBy || 'system' },
                { key: 'created', header: 'Created', render: (r) => new Date(r.createdAt).toLocaleString() },
                {
                  key: 'actions',
                  header: '',
                  render: (r) => (
                    <Link href={`/app/diff?capability=${id}&version=${r.version}`} className="text-xs text-brand-highlight hover:underline">
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
              nodes={nodesForManifest(recordOf(manifest.data?.manifest))}
              edges={edgesForManifest(recordOf(manifest.data?.manifest))}
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
              rowKey={(r) => r.id}
              onRowClick={(r) => { router.push(`/app/releases/${r.id}`); }}
              columns={[
                { key: 'v', header: 'Version', render: (r) => `v${r.capabilityVersion}` },
                { key: 'env', header: 'Environment', render: (r) => <span className="font-mono text-xs">{r.environment}</span> },
                { key: 'state', header: 'State', render: (r) => <StatusPill kind={statusKindOf(r.status)} /> },
                { key: 'hash', header: 'Identifier', render: (r) => <HashChip hash={r.id} /> },
                { key: 'canary', header: 'Canary', render: (r) => `${r.canaryPercent}%` },
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

function recordOf(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
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
