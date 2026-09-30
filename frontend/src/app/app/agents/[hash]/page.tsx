'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Bot } from 'lucide-react';
import { use } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, workspaceApi, type WorkspaceRow } from '@/lib/api';
import { HashChip } from '@/components/brand/hash-chip';
import { PageHeader } from '@/components/brand/page-header';
import { QueryError } from '@/components/brand/query-error';
import { Surface } from '@/components/brand/surface';

export default function AgentRevisionPage({ params }: { params: Promise<{ hash: string }> }) {
  const session = useRequireSession();
  const { hash } = use(params);
  const workspaces = useQuery<WorkspaceRow[]>({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1, 100).then((response) => response.data), enabled: Boolean(session) });
  const workspaceId = workspaces.data?.[0]?.id;
  const revision = useQuery({ queryKey: ['agent-specification', workspaceId, hash], queryFn: () => agentSpecificationApi.get(workspaceId!, hash), enabled: Boolean(session && workspaceId) });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (revision.isError) return <QueryError message={revision.error} onRetry={() => void revision.refetch()} />;
  const data = revision.data?.data as { specification?: { role?: string; objective?: string }; status?: string; author?: string; changeReason?: string; parentHash?: string | null } | undefined;

  return <div className="space-y-6"><Link href="/app/agents" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"><ArrowLeft className="h-3 w-3" /> Agent specifications</Link><PageHeader eyebrow="Agent revision" title={data?.specification?.role ?? 'Loading revision'} subtitle={data?.specification?.objective} /><Surface><div className="flex items-center gap-3"><Bot className="h-5 w-5 text-brand-highlight" /><div className="text-sm text-text-muted">{data?.status ?? 'Loading'} · {data?.author ?? '—'} · {data?.changeReason ?? '—'}</div></div><pre className="mt-6 max-h-[32rem] overflow-auto rounded-xl border border-border-subtle bg-surface-2 p-4 text-xs text-text-muted">{JSON.stringify(data, null, 2)}</pre>{data?.parentHash && <div className="mt-4 text-xs text-text-subtle">Parent revision <HashChip hash={data.parentHash} /></div>}</Surface></div>;
}
