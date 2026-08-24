'use client';

import { Suspense } from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useMemo, useState } from 'react';
import {
  ArrowLeft, GitCompareArrows, ScrollText,
} from 'lucide-react';
import { versionApi, releaseApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { HashChip } from '@/components/brand/hash-chip';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/brand/empty-state';
import { cn } from '@/lib/utils';

type Mode = 'unified' | 'split';

function DiffPageInner() {
  const session = useRequireSession();
  const search = useSearchParams();
  const capabilityParam = search.get('capability') ?? '';
  const initialFrom = search.get('from') ?? '';
  const initialTo = search.get('to') ?? '';
  const [mode, setMode] = useState<Mode>('split');

  const fromVer = useQuery({
    queryKey: ['versions', capabilityParam],
    queryFn: () => capabilityParam ? versionApi.list(capabilityParam).then((r) => r.data).catch(() => []) : Promise.resolve([]),
    enabled: Boolean(capabilityParam) && Boolean(session),
  });

  const versions = (Array.isArray(fromVer.data) ? fromVer.data : []) as Array<Record<string, unknown>>;

  const [fromId, setFromId] = useState<string>(initialFrom);
  const [toId, setToId] = useState<string>(initialTo);

  const fromData = useQuery({
    queryKey: ['manifest', fromId],
    queryFn: () => fromId ? releaseApi.get(fromId).then((r) => r.data).catch(() => null) : Promise.resolve(null),
    enabled: Boolean(fromId) && Boolean(session),
  });
  const toData = useQuery({
    queryKey: ['manifest', toId],
    queryFn: () => toId ? releaseApi.get(toId).then((r) => r.data).catch(() => null) : Promise.resolve(null),
    enabled: Boolean(toId) && Boolean(session),
  });

  const fromText = useMemo(() => stringifyManifest(fromData.data), [fromData.data]);
  const toText = useMemo(() => stringifyManifest(toData.data), [toData.data]);

  const unifiedLines = useMemo(() => unifiedDiff(fromText, toText), [fromText, toText]);

  const fromHash = (fromData.data as { manifestHash?: string } | null | undefined)?.manifestHash ?? '';
  const toHash = (toData.data as { manifestHash?: string } | null | undefined)?.manifestHash ?? '';

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/capabilities" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Registry
        </Link>
        <PageHeader
          eyebrow="Diff / provenance"
          title="Compare capability versions"
          subtitle="What changed, who changed it, why it matters, and a clean lineage back to source."
        />
      </div>

      <Surface>
        <SurfaceHeader
          title="Pick two manifests"
          description="Either pick a capability then two of its versions, or paste manifest hashes directly."
          actions={
            <div className="flex items-center gap-1 rounded-md border border-border-subtle bg-surface-2 p-0.5 text-xs">
              <button
                onClick={() => setMode('split')}
                className={cn('px-2.5 py-1 rounded-md', mode === 'split' ? 'bg-surface-1 text-text-strong' : 'text-text-muted')}
              >Split</button>
              <button
                onClick={() => setMode('unified')}
                className={cn('px-2.5 py-1 rounded-md', mode === 'unified' ? 'bg-surface-1 text-text-strong' : 'text-text-muted')}
              >Unified</button>
            </div>
          }
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <SourcePicker
            label="From"
            versions={versions}
            value={fromId}
            onChange={setFromId}
          />
          <SourcePicker
            label="To"
            versions={versions}
            value={toId}
            onChange={setToId}
          />
        </div>
      </Surface>

      {!fromId || !toId ? (
        <EmptyState
          icon={GitCompareArrows}
          title="Choose two versions to diff"
          description="Diff is semantic — prompt blocks, policies, tools, memory, guardrails, MCP servers, and schedules."
        />
      ) : mode === 'split' ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <DiffPanel label="From" text={fromText} hash={fromHash} />
          <DiffPanel label="To" text={toText} hash={toHash} />
        </div>
      ) : (
        <Surface>
          <SurfaceHeader title="Unified diff" description="+ added · − removed · ~ modified" />
          <div className="rounded-lg border border-border-subtle bg-surface-0 overflow-hidden">
            <ScrollText className="ml-3 mt-3 h-3.5 w-3.5 text-text-subtle inline" />
            <pre className="overflow-x-auto px-3 py-3 text-xs font-mono leading-relaxed">
              {unifiedLines.map((line) => (
                <DiffLine key={line.id} type={line.type} text={line.text} />
              ))}
            </pre>
          </div>
        </Surface>
      )}
    </div>
  );
}

function SourcePicker({
  label,
  versions,
  value,
  onChange,
}: {
  label: string;
  versions: Array<Record<string, unknown>>;
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <div>
      <label className="block text-xs font-medium uppercase tracking-wider text-text-subtle">{label}</label>
      <div className="mt-2">
        <ThemedSelect
          value={value || undefined}
          onValueChange={onChange}
          placeholder="Choose a version…"
          options={versions.map((v) => {
            const id = String(v['id']);
            const vNum = String(v['version'] ?? '?');
            const h = String(v['manifestHash'] ?? v['id']);
            return { value: id, label: `v${vNum} · ${h.slice(0, 8)}` };
          })}
          ariaLabel={label}
          triggerClassName="w-full"
        />
      </div>
    </div>
  );
}

function DiffPanel({ label, text, hash }: { label: string; text: string; hash?: string }) {
  return (
    <Surface>
      <SurfaceHeader
        title={label}
        description={hash ? <HashChip hash={hash} /> : 'No manifest content available.'}
      />
      <pre className="max-h-[480px] overflow-auto rounded-md border border-border-subtle bg-surface-0 p-4 text-xs leading-relaxed font-mono">
        {text}
      </pre>
    </Surface>
  );
}

interface DiffLineKind { id: number; type: 'add' | 'remove' | 'context'; text: string }

function DiffLine({ type, text }: { type: DiffLineKind['type']; text: string }) {
  const cls =
    type === 'add' ? 'text-success' :
    type === 'remove' ? 'text-destructive' :
    'text-text-muted';
  const prefix = type === 'add' ? '+ ' : type === 'remove' ? '− ' : '  ';
  return (
    <div className={cls}>
      <span className="mr-3 opacity-50 select-none">{prefix}</span>
      <span>{text}</span>
    </div>
  );
}

function stringifyManifest(data: unknown): string {
  if (!data) return '';
  if (typeof data === 'string') return data;
  try {
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
}

function unifiedDiff(fromText: string, toText: string): DiffLineKind[] {
  const a = fromText.split('\n');
  const b = toText.split('\n');
  const setA = new Map<string, number>();
  a.forEach((line, i) => setA.set(line, i));
  const at = (arr: string[], idx: number): string => arr[idx] ?? '';
  const out: DiffLineKind[] = [];
  let i = 0;
  let j = 0;
  let k = 0;
  while (j < b.length) {
    if (i < a.length && at(a, i) === at(b, j)) {
      out.push({ id: k++, type: 'context', text: at(b, j) });
      i++; j++;
    } else if (!setA.has(at(b, j))) {
      out.push({ id: k++, type: 'add', text: at(b, j) });
      j++;
    } else if (i < a.length) {
      out.push({ id: k++, type: 'remove', text: at(a, i) });
      i++;
    } else {
      out.push({ id: k++, type: 'add', text: at(b, j) });
      j++;
    }
  }
  while (i < a.length) {
    out.push({ id: k++, type: 'remove', text: at(a, i++) });
  }
  return out;
}

export default function DiffPage() {
  return (
    <Suspense fallback={<div className="p-6 text-sm text-text-muted">Loading…</div>}>
      <DiffPageInner />
    </Suspense>
  );
}
