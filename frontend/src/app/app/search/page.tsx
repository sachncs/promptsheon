'use client';

import { useDeferredValue } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Search as SearchIcon } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { searchApi, type SearchResult } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { HashChip } from '@/components/brand/hash-chip';
import { Input } from '@/components/ui/input';
import { QueryError } from '@/components/brand/query-error';

export default function SearchPage() {
  const session = useRequireSession();
  const router = useRouter();
  const searchParams = useSearchParams();
  const q = searchParams.get('q') ?? '';
  const deferredQuery = useDeferredValue(q.trim());

  const results = useQuery({
    queryKey: ['search', deferredQuery],
    queryFn: () => searchApi.q(deferredQuery),
    enabled: deferredQuery.length >= 2,
  });

  const rows: SearchResult[] = results.data ?? [];

  if (!session) return null;
  if (results.isError) return <QueryError message={results.error} onRetry={() => void results.refetch()} />;

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Search" title="Full-text search" subtitle="FTS5 over manifests and audit entries. Matches highlighted." />
      <Surface padded>
        <div className="relative">
          <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-subtle" />
          <Input
            autoFocus
            aria-label="Search capabilities, releases, and audit entries"
            placeholder="Search capability names, release notes, audit messages…"
            value={q}
            onChange={(e) => {
              const nextParams = new URLSearchParams(searchParams.toString());
              const value = e.target.value;
              if (value) nextParams.set('q', value);
              else nextParams.delete('q');
              const query = nextParams.toString();
              router.replace(query ? `/app/search?${query}` : '/app/search', { scroll: false });
            }}
            className="pl-9 h-9 bg-surface-1 border-border-subtle"
          />
        </div>
        <div className="mt-5" aria-live="polite">
          {deferredQuery.length < 2 ? (
            <p className="text-text-muted text-sm">Type at least two characters.</p>
          ) : results.isFetching ? (
            <p className="text-text-muted text-sm">Searching…</p>
          ) : rows.length === 0 ? (
            <EmptyState
              title="No matches"
              description={`Nothing matches "${q}".`}
              icon={SearchIcon}
              className="border-0 bg-transparent p-8"
            />
          ) : (
            <ul className="divide-y divide-border-subtle">
              {rows.map((r) => (
                <li key={`${r.kind}-${r.resourceId}`} className="py-3">
                  <div className="flex items-center gap-2">
                    <span className="text-xs uppercase tracking-wider text-text-subtle">{r.kind}</span>
                    <HashChip hash={r.resourceId} length={16} />
                  </div>
                  <div className="mt-1 text-sm text-text-strong">{r.title}</div>
                  <div className="mt-1 line-clamp-2 text-xs text-text-muted">{r.body}</div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Surface>
    </div>
  );
}
