'use client';

import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Play, ArrowLeft, Clock, DollarSign, Hash, Activity, RotateCcw } from 'lucide-react';
import { executionApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { QueryError } from '@/components/brand/query-error';

export default function ExecutionDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;
  const router = useRouter();
  const queryClient = useQueryClient();

  const detail = useQuery({
    queryKey: ['execution', id],
    queryFn: () => executionApi.get(id).then((r) => r.data),
    enabled: Boolean(id),
    retry: false,
  });

  const replayMutation = useMutation({
    mutationFn: () => executionApi.replay(id),
    onSuccess: (response) => {
      const replayId = response.data?.replayExecutionId;
      if (replayId) {
        queryClient.invalidateQueries({ queryKey: ['execution', id] });
        queryClient.invalidateQueries({ queryKey: ['execution', replayId] });
        router.push(`/app/executions/${replayId}`);
      }
    },
  });

  if (!session) return null;

  const data = detail.data;
  const isError = detail.isError;
  const canReplay = data?.replayOf === null;
  const inputs = data ? parseJson(data.inputs) : null;
  const outputs = data ? parseJson(data.outputs) : null;
  const status = data ? (data.error ? 'error' : 'completed') : undefined;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/releases" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="size-3" /> Back to releases
        </Link>
      </div>

      <PageHeader
        eyebrow="Execution"
        title={`Execution ${id.slice(0, 12)}…`}
        subtitle={
          data?.replayOf
            ? `Replay of execution ${data.replayOf.slice(0, 12)}… — same model, environment, and inputs.`
            : 'A single invocation of a capability. Inputs, outputs, and trace.'
        }
        actions={
          <div className="flex items-center gap-2">
            {data?.inputHash ? <HashChip hash={data.inputHash} /> : null}
            {canReplay ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={replayMutation.isPending || !data?.id}
                onClick={() => replayMutation.mutate()}
              >
                <RotateCcw className="size-3.5" />
                {replayMutation.isPending ? 'Replaying…' : 'Replay'}
              </Button>
            ) : null}
          </div>
        }
      />

      {isError ? (
        <QueryError message={detail.error} onRetry={() => void detail.refetch()} />
      ) : detail.isLoading ? (
        <Surface>
          <div className="text-sm text-text-muted">Loading execution…</div>
        </Surface>
      ) : !data ? (
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
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-4">
            <StatCard
              label="Status"
              value={status ?? 'unknown'}
              icon={Activity}
              hint={new Date(data.timestamp).toLocaleString()}
            />
            <StatCard
              label="Duration"
              value={`${(data.latencyMs / 1000).toFixed(2)}s`}
              icon={Clock}
              hint={data.environment}
            />
            <StatCard
              label="Cost"
              value={`$${data.costUsd.toFixed(4)}`}
              icon={DollarSign}
              hint="micros / 1,000,000"
            />
            <StatCard
              label="Manifest"
              value={data.capabilityVersionId ? data.capabilityVersionId.slice(0, 12) + '…' : '—'}
              icon={Hash}
              hint={`${data.provider}/${data.model}`}
            />
          </div>

          {(data.replayCount !== undefined && data.replayCount > 0) || data.replayOf ? (
            <div className="flex flex-wrap items-center gap-3 rounded-md border border-border-subtle bg-surface-1 px-4 py-2 text-xs text-text-muted">
              {data.replayCount !== undefined && data.replayCount > 0 ? (
                <span>
                  Replayed <strong className="text-text-default">{data.replayCount}</strong>{' '}
                  {data.replayCount === 1 ? 'time' : 'times'}
                </span>
              ) : null}
              {data.replayOf ? (
                <Link
                  href={`/app/executions/${data.replayOf}`}
                  className="text-text-default underline-offset-2 hover:underline"
                >
                  View original execution
                </Link>
              ) : null}
            </div>
          ) : null}

          {status && (
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider text-text-subtle">State</span>
              <StatusPill kind={statusKindOf(status)} />
            </div>
          )}

          <div className="grid gap-5 lg:grid-cols-2">
            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Inputs" description="What was passed to the capability." />
              <pre className="mx-5 mb-5 max-h-96 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                {inputs ? JSON.stringify(inputs, null, 2) : '(no inputs recorded)'}
              </pre>
            </Surface>

            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Outputs" description="What the capability returned." />
              <pre className="mx-5 mb-5 max-h-96 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                {outputs ? JSON.stringify(outputs, null, 2) : '(no outputs yet)'}
              </pre>
            </Surface>
          </div>

          {data.error && (
            <Surface>
              <div className="text-sm font-semibold text-destructive">Error</div>
              <p className="mt-1 text-sm text-text-default">{data.error}</p>
            </Surface>
          )}
        </>
      )}
    </div>
  );
}

function parseJson(value: string): unknown {
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return value;
  }
}
