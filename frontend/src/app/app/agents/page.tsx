'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Bot, ChevronLeft, ChevronRight, Plus } from 'lucide-react';
import { useState } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, workspaceApi, type AgentSpecificationStatus, type WorkspaceRow } from '@/lib/api';
import { EmptyState } from '@/components/brand/empty-state';
import { HashChip } from '@/components/brand/hash-chip';
import { PageHeader } from '@/components/brand/page-header';
import { QueryError } from '@/components/brand/query-error';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { Surface } from '@/components/brand/surface';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';

const PAGE_SIZE = 20;

export default function AgentsPage() {
  const session = useRequireSession();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState<AgentSpecificationStatus | 'all'>('all');
  const workspaces = useQuery<WorkspaceRow[]>({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1, 100).then((response) => response.data),
    enabled: Boolean(session),
  });
  const workspaceId = workspaces.data?.[0]?.id;
  const specifications = useQuery({
    queryKey: ['agent-specifications', workspaceId, page, status],
    queryFn: () => agentSpecificationApi.list(workspaceId!, { page, pageSize: PAGE_SIZE, ...(status !== 'all' ? { status } : {}) }).then((response) => response.data),
    enabled: Boolean(session && workspaceId),
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (specifications.isError) return <QueryError message={specifications.error} onRetry={() => void specifications.refetch()} />;

  const data = specifications.data;
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE));

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Build / Agents"
        title="Agent specifications"
        subtitle="Immutable, content-addressed agent definitions with full revision lineage and lifecycle state."
        actions={(
          <Button asChild>
            <Link href="/app/agents/new"><Plus className="h-4 w-4" /> New specification</Link>
          </Button>
        )}
      />

      {!workspaceId ? (
        <EmptyState
          icon={Bot}
          title="Create a workspace first"
          description="Agent specifications belong to a workspace. Create one before defining your first agent."
          action={<Button asChild variant="outline"><Link href="/app/workspaces">Open workspaces</Link></Button>}
        />
      ) : data?.total === 0 ? (
        <EmptyState
          icon={Bot}
          title="No agent specifications yet"
          description="Start with an immutable specification that captures the agent's role, policies, tools, memory, and evaluation rules."
          action={<Button asChild><Link href="/app/agents/new"><Plus className="h-4 w-4" /> Create specification</Link></Button>}
        />
      ) : (
        <Surface padded={false}>
          <div className="flex flex-col gap-3 border-b border-border-subtle p-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm text-text-muted">{data?.total ?? 0} revision{data?.total === 1 ? '' : 's'}</div>
            <ThemedSelect
              value={status}
              onValueChange={(value) => { setStatus(value as AgentSpecificationStatus | 'all'); setPage(1); }}
              options={[{ value: 'all', label: 'All statuses' }, { value: 'draft', label: 'Draft' }, { value: 'candidate', label: 'Candidate' }, { value: 'published', label: 'Published' }, { value: 'retired', label: 'Retired' }]}
              ariaLabel="Filter agent specifications by status"
              triggerClassName="w-full sm:w-44"
            />
          </div>
          <div className="divide-y divide-border-subtle">
            {(data?.items ?? []).map((item) => (
              <div key={item.hash} className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <HashChip hash={item.hash} length={14} />
                    <StatusPill kind={statusKindOf(item.status)} label={item.status} />
                  </div>
                  <p className="mt-2 truncate text-sm font-medium text-text-strong">{item.changeReason}</p>
                  <p className="mt-1 text-xs text-text-subtle">{item.author} · {formatDate(item.createdAt)} · schema {item.schemaVersion}</p>
                </div>
                <Link href={`/app/agents/${item.hash}`} className="text-sm font-medium text-brand-highlight hover:underline">Inspect revision</Link>
              </div>
            ))}
          </div>
          <div className="flex items-center justify-between border-t border-border-subtle p-4 text-sm text-text-muted">
            <span>Page {page} of {totalPages}</span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}><ChevronLeft className="h-4 w-4" /> Previous</Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((current) => current + 1)}>Next <ChevronRight className="h-4 w-4" /></Button>
            </div>
          </div>
        </Surface>
      )}
    </div>
  );
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}
