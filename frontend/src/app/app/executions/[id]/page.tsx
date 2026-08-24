'use client';

import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Play, ArrowLeft, Clock, DollarSign, Hash, Activity } from 'lucide-react';
import { executionApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

interface ExecutionDetail {
  id: string;
  capabilityId?: string;
  capabilityVersionId?: string;
  capabilityName?: string;
  manifestHash?: string;
  inputs?: Record<string, unknown>;
  outputs?: Record<string, unknown>;
  status?: string;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  costMicros?: number;
  trace?: Array<{ node: string; at: string; event: string; data?: Record<string, unknown> }>;
  error?: { message: string; stack?: string };
}

export default function ExecutionDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;

  const detail = useQuery({
    queryKey: ['execution', id],
    queryFn: () => executionApi.get(id).then((r) => r.data as ExecutionDetail),
    enabled: Boolean(id),
    retry: false,
  });

  if (!session) return null;

  const data = detail.data;
  const isError = detail.isError;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/releases" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="size-3" /> Back to releases
        </Link>
      </div>

      <PageHeader
        eyebrow="Execution"
        title={data?.capabilityName ? `${data.capabilityName} execution` : `Execution ${id.slice(0, 12)}…`}
        subtitle="A single invocation of a capability. Inputs, outputs, and trace."
        actions={data?.manifestHash ? <HashChip hash={data.manifestHash} /> : undefined}
      />

      {isError ? (
        <EmptyState
          icon={Play}
          title="Execution not found"
          description={`No execution matches ${id.slice(0, 16)}. Trigger a run from an active release to populate this view.`}
          action={
            <Link href="/app/releases">
              <Button>Open releases</Button>
            </Link>
          }
        />
      ) : !data ? (
        <Surface>
          <div className="text-sm text-text-muted">{detail.isLoading ? 'Loading execution…' : 'No execution data.'}</div>
        </Surface>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-4">
            <StatCard
              label="Status"
              value={data.status ?? 'unknown'}
              icon={Activity}
              hint={data.completedAt ? new Date(data.completedAt).toLocaleString() : (data.startedAt ? `started ${new Date(data.startedAt).toLocaleString()}` : '—')}
            />
            <StatCard
              label="Duration"
              value={data.durationMs !== undefined ? `${(data.durationMs / 1000).toFixed(2)}s` : '—'}
              icon={Clock}
              hint={data.startedAt ? new Date(data.startedAt).toLocaleString() : undefined}
            />
            <StatCard
              label="Cost"
              value={data.costMicros !== undefined ? `$${(data.costMicros / 1_000_000).toFixed(4)}` : '—'}
              icon={DollarSign}
              hint="micros / 1,000,000"
            />
            <StatCard
              label="Manifest"
              value={data.manifestHash ? data.manifestHash.slice(0, 12) + '…' : '—'}
              icon={Hash}
              hint={data.capabilityVersionId ? `version ${data.capabilityVersionId.slice(0, 8)}` : undefined}
            />
          </div>

          {data.status && (
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider text-text-subtle">State</span>
              <StatusPill kind={(data.status as never) ?? 'neutral'} />
            </div>
          )}

          <div className="grid gap-5 lg:grid-cols-2">
            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Inputs" description="What was passed to the capability." />
              <pre className="mx-5 mb-5 max-h-96 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                {data.inputs ? JSON.stringify(data.inputs, null, 2) : '(no inputs recorded)'}
              </pre>
            </Surface>

            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Outputs" description="What the capability returned." />
              <pre className="mx-5 mb-5 max-h-96 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                {data.outputs ? JSON.stringify(data.outputs, null, 2) : '(no outputs yet)'}
              </pre>
            </Surface>
          </div>

          {data.trace && data.trace.length > 0 && (
            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Trace" description="Per-node events captured during execution." />
              <ol className="divide-y divide-border-subtle">
                {data.trace.map((t, i) => (
                  <li key={i} className="grid grid-cols-12 gap-3 px-5 py-3 text-sm">
                    <span className="col-span-2 font-mono text-xs text-text-subtle">{new Date(t.at).toLocaleTimeString()}</span>
                    <span className="col-span-3 font-medium text-text-strong">{t.node}</span>
                    <span className="col-span-7 text-text-muted">{t.event}{t.data ? ` — ${JSON.stringify(t.data)}` : ''}</span>
                  </li>
                ))}
              </ol>
            </Surface>
          )}

          {data.error && (
            <Surface>
              <div className="text-sm font-semibold text-destructive">Error</div>
              <p className="mt-1 text-sm text-text-default">{data.error.message}</p>
              {data.error.stack && (
                <pre className="mt-3 max-h-48 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs text-text-muted">
                  {data.error.stack}
                </pre>
              )}
            </Surface>
          )}
        </>
      )}
    </div>
  );
}