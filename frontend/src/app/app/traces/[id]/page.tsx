'use client';

import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, AlertCircle, Clock, Cpu, DollarSign, GitBranch, type LucideIcon } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { traceApi, type TraceSpan } from '@/lib/api';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';

export default function TraceDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? '';

  const trace = useQuery({
    queryKey: ['trace', id],
    queryFn: () => traceApi.get(id),
    enabled: Boolean(session && id),
  });

  if (!session) return null;
  const data = trace.data;

  if (trace.isError || !data) {
    return (
      <div className="space-y-6">
        <BackLink />
        <EmptyState
          icon={AlertCircle}
          title="Trace not available"
          description={`No trace found for ${id.slice(0, 16)}.`}
        />
      </div>
    );
  }

  const { run, spans } = data;
  const startedMs = Date.parse(run.startTime);
  const endedMs = run.endTime ? Date.parse(run.endTime) : Date.now();
  const durationMs = Math.max(0, endedMs - startedMs);

  // Build span tree: top-level spans are root nodes; everything
  // else hangs under its parentSpanId.
  const childrenByParent = new Map<string | null, TraceSpan[]>();
  for (const s of spans) {
    const arr = childrenByParent.get(s.parentSpanId) ?? [];
    arr.push(s);
    childrenByParent.set(s.parentSpanId, arr);
  }
  const roots = childrenByParent.get(null) ?? [];
  const sortByStart = (a: TraceSpan, b: TraceSpan) =>
    Date.parse(a.startTime) - Date.parse(b.startTime);
  roots.sort(sortByStart);
  for (const arr of childrenByParent.values()) arr.sort(sortByStart);

  return (
    <div className="space-y-6">
      <BackLink />

      <div>
        <div className="text-xs uppercase tracking-wider text-text-subtle">Trace</div>
        <h1 className="mt-1 flex items-baseline gap-3 text-2xl font-semibold text-text-strong">
          <span className="font-mono">{run.name}</span>
          <StatusPill
            kind={run.status === 'success' ? 'active' : run.status === 'error' ? 'error' : 'review'}
            label={run.status}
          />
          <span className="text-xs text-text-subtle">{run.environment}</span>
        </h1>
        <p className="mt-1 text-sm text-text-muted">
          Started {new Date(run.startTime).toLocaleString()}
          {run.endTime ? ` · ended ${new Date(run.endTime).toLocaleString()}` : ' · still running'}
        </p>
      </div>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Run summary" />
        <dl className="grid grid-cols-2 gap-x-6 gap-y-3 px-5 pb-5 text-sm md:grid-cols-4">
          <SummaryCell label="Duration" value={`${durationMs} ms`} Icon={Clock} />
          <SummaryCell label="Tokens" value={run.totalTokens.toLocaleString()} Icon={Cpu} />
          <SummaryCell label="Cost" value={`$${run.totalCostUsd.toFixed(4)}`} Icon={DollarSign} />
          <SummaryCell label="Spans" value={String(spans.length)} Icon={GitBranch} />
        </dl>
        {run.executionId && (
          <div className="border-t border-border-subtle px-5 py-3 text-xs text-text-muted">
            executionId <HashChip hash={run.executionId} length={24} />
          </div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Span tree" description={`${roots.length} root span(s).`} />
        <ul className="px-5 pb-5">
          {roots.map((s) => (
            <SpanNode key={s.id} span={s} children={childrenByParent.get(s.id) ?? []} depth={0} />
          ))}
        </ul>
      </Surface>
    </div>
  );
}

function SpanNode({
  span,
  children,
  depth,
}: {
  span: TraceSpan;
  children: TraceSpan[];
  depth: number;
}) {
  const startMs = Date.parse(span.startTime);
  const endMs = span.endTime ? Date.parse(span.endTime) : Date.now();
  const ms = Math.max(0, endMs - startMs);
  return (
    <li>
      <div
        className="flex items-start gap-3 border-l border-border-subtle py-2 pr-3"
        style={{ paddingLeft: depth * 18 + 8 }}
      >
      <span
        className="mt-0.5 inline-block size-2 shrink-0 rounded-full bg-brand"
        aria-hidden="true"
      />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-2">
            <span className="font-mono text-sm text-text-strong">{span.name}</span>
            <span className="rounded-full bg-surface-2 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-text-muted">
              {span.kind}
            </span>
            <span className="text-xs text-text-subtle">{ms} ms</span>
            {span.model && (
              <span className="text-xs text-text-muted">· {span.model}</span>
            )}
            {typeof span.totalTokens === 'number' && span.totalTokens > 0 && (
              <span className="text-xs text-text-muted">· {span.totalTokens.toLocaleString()} tokens</span>
            )}
            {typeof span.costUsd === 'number' && span.costUsd > 0 && (
              <span className="text-xs text-text-muted">· ${span.costUsd.toFixed(4)}</span>
            )}
          </div>
          {span.inputText && (
            <details className="mt-1 text-xs">
              <summary className="cursor-pointer text-text-muted hover:text-text-default">
                input
              </summary>
              <pre className="mt-1 max-h-40 overflow-auto rounded-md bg-surface-0 p-2 font-mono text-[11px] leading-relaxed text-text-default">
                {span.inputText}
              </pre>
            </details>
          )}
          {span.outputText && (
            <details className="mt-1 text-xs">
              <summary className="cursor-pointer text-text-muted hover:text-text-default">
                output
              </summary>
              <pre className="mt-1 max-h-40 overflow-auto rounded-md bg-surface-0 p-2 font-mono text-[11px] leading-relaxed text-text-default">
                {span.outputText}
              </pre>
            </details>
          )}
        </div>
      </div>
      {children.length > 0 && (
        <ul>
          {children.map((c) => (
            <SpanNode key={c.id} span={c} children={[]} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  );
}

function SummaryCell({ label, value, Icon }: { label: string; value: string; Icon: LucideIcon }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wider text-text-subtle">{label}</dt>
      <dd className="mt-1 flex items-center gap-1 font-mono text-text-strong">
        <Icon className="h-3 w-3" aria-hidden="true" />
        {value}
      </dd>
    </div>
  );
}

function BackLink() {
  return (
    <div>
      <Link
        href="/app/traces"
        className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
      >
        <ArrowLeft className="h-3 w-3" /> Back to traces
      </Link>
    </div>
  );
}
