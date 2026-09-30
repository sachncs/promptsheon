'use client';

import { useState } from 'react';
import { Download, FileSearch, Filter } from 'lucide-react';
import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { evidenceApi, type EvidenceRecord, workspaceApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';
import { StatusPill } from '@/components/brand/status-pill';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';

const eventOptions = [
  { value: 'all', label: 'All events' },
  { value: 'execution.started', label: 'Execution started' },
  { value: 'execution.completed', label: 'Execution completed' },
  { value: 'execution.failed', label: 'Execution failed' },
  { value: 'model.called', label: 'Model called' },
  { value: 'tool.called', label: 'Tool called' },
  { value: 'error.observed', label: 'Error observed' },
];

export default function EvidencePage() {
  const session = useRequireSession();
  const [eventType, setEventType] = useState('');
  const [workspaceId, setWorkspaceId] = useState('');
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1, 100).then((response) => response.data),
    enabled: Boolean(session),
  });
  const evidence = useInfiniteQuery({
    queryKey: ['evidence', { eventType, workspaceId }],
    queryFn: ({ pageParam }) => evidenceApi.list({
      limit: 100,
      ...(pageParam ? { before: pageParam } : {}),
      ...(eventType ? { eventType } : {}),
      ...(workspaceId ? { workspaceId } : {}),
    }).then((r) => r.data),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.items.length === 100
      ? lastPage.items[lastPage.items.length - 1]?.occurredAt
      : undefined,
    enabled: Boolean(session),
    refetchInterval: 15_000,
  });
  const exportMutation = useMutation({
    mutationFn: () => evidenceApi.export({ limit: 500, ...(eventType ? { eventType } : {}), ...(workspaceId ? { workspaceId } : {}) }),
    onSuccess: (exported) => {
      const blob = new Blob([JSON.stringify(exported, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'promptsheon-evidence.json';
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
    },
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (evidence.isError) return <QueryError message={evidence.error} onRetry={() => void evidence.refetch()} />;

  const pages = evidence.data?.pages ?? [];
  const items = pages.flatMap((page) => page.items);

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Observability"
        title="Evidence"
        subtitle="Immutable, redacted decision records across your organisation. Trace every execution from admission to outcome."
        actions={
          <Button variant="outline" size="sm" onClick={() => exportMutation.mutate()} disabled={evidence.isPending || exportMutation.isPending}>
            <Download className="mr-1.5 h-3.5 w-3.5" /> {exportMutation.isPending ? 'Exporting…' : 'Export JSON'}
          </Button>
        }
      />
      {exportMutation.isError ? (
        <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          The evidence export could not be generated. Try again.
        </div>
      ) : exportMutation.isSuccess ? (
        <div role="status" className="rounded-lg border border-success/30 bg-success/5 px-4 py-3 text-sm text-success">
          Evidence export downloaded.
        </div>
      ) : null}

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Evidence timeline"
          description={evidence.data ? `${items.length} record(s) loaded in the current view.` : 'Loading immutable records…'}
          actions={
            <div className="flex items-center gap-2">
              <Filter className="h-3.5 w-3.5 text-text-subtle" aria-hidden="true" />
              <ThemedSelect
                value={workspaceId || 'all'}
                onValueChange={(value) => setWorkspaceId(value === 'all' ? '' : value)}
                options={[{ value: 'all', label: 'All workspaces' }, ...(workspaces.data ?? []).map((workspace) => ({ value: workspace.id, label: workspace.name }))]}
                ariaLabel="Filter evidence by workspace"
              />
              <ThemedSelect value={eventType || 'all'} onValueChange={(value) => setEventType(value === 'all' ? '' : value)} options={eventOptions} />
            </div>
          }
        />
        {evidence.isPending ? (
          <div className="px-5 py-12 text-sm text-text-muted" aria-busy="true">Loading evidence…</div>
        ) : items.length === 0 ? (
          <EmptyState
            className="m-5 border-0 bg-transparent shadow-none p-12"
            icon={FileSearch}
            title="No evidence yet"
            description="Run a capability or choose another event filter. Redaction happens before evidence is persisted."
          />
        ) : (
          <ul className="divide-y divide-border-subtle">
            {items.map((item) => <EvidenceRow key={item.id} item={item} />)}
          </ul>
        )}
        {evidence.hasNextPage ? (
          <div className="border-t border-border-subtle px-5 py-3">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void evidence.fetchNextPage()}
              disabled={evidence.isFetchingNextPage}
            >
              {evidence.isFetchingNextPage ? 'Loading older records…' : 'Load older records'}
            </Button>
          </div>
        ) : null}
      </Surface>
    </div>
  );
}

function EvidenceRow({ item }: { item: EvidenceRecord }) {
  const status = item.eventType.endsWith('failed') || item.eventType === 'error.observed' ? 'error' : 'active';
  return (
    <li className="px-5 py-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <StatusPill kind={status} label={item.eventType} />
        {item.executionId ? <span className="font-mono text-xs text-text-muted">execution:{item.executionId.slice(0, 12)}…</span> : null}
        {item.stepId ? <span className="font-mono text-xs text-text-muted">step:{item.stepId}</span> : null}
        <time className="ml-auto text-xs text-text-subtle" dateTime={item.occurredAt}>{new Date(item.occurredAt).toLocaleString()}</time>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-text-subtle">
        <span>retention: {item.retentionClass}</span>
        <span>·</span>
        <span className="font-mono">hash:{item.payloadHash.slice(0, 16)}…</span>
      </div>
      <details className="mt-2">
        <summary className="cursor-pointer text-xs text-text-muted hover:text-text-default">View redacted payload</summary>
        <pre className="mt-1 max-h-40 overflow-auto rounded-md bg-surface-0 p-2 font-mono text-[11px] leading-relaxed text-text-default">{JSON.stringify(item.payload, null, 2)}</pre>
      </details>
    </li>
  );
}
