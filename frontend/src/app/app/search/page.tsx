'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search as SearchIcon } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { searchApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { HashChip } from '@/components/brand/hash-chip';
import { Input } from '@/components/ui/input';

export default function SearchPage() {
  const session = useRequireSession();
  const [q, setQ] = useState('');

  const results = useQuery({
    queryKey: ['search', q],
    queryFn: () => (q.length >= 2 ? searchApi.q(q) : Promise.resolve([])),
    enabled: q.length >= 2,
  });

  const rows = (results.data ?? []) as Array<Record<string, unknown>>;

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Search" title="Full-text search" subtitle="FTS5 over manifests and audit entries. Matches highlighted." />
      <Surface padded>
        <div className="relative">
          <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-subtle" />
          <Input
            autoFocus
            placeholder="Search capability names, release notes, audit messages…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            className="pl-9 h-9 bg-surface-1 border-border-subtle"
          />
        </div>
        <div className="mt-5">
          {q.length < 2 ? (
            <p className="text-text-muted text-sm">Type at least two characters.</p>
          ) : rows.length === 0 ? (
            <EmptyState
              title="No matches"
              description={`Nothing matches "${q}".`}
              icon={SearchIcon}
              className="border-0 bg-transparent p-8"
            />
          ) : (
            <ul className="divide-y divide-border-subtle">
              {rows.map((r, i) => (
                <li key={String(r['resource_id']) ?? i} className="py-3">
                  <div className="flex items-center gap-2">
                    <span className="text-xs uppercase tracking-wider text-text-subtle">{String(r['kind'])}</span>
                    <HashChip hash={String(r['resource_id'] ?? '')} length={16} />
                  </div>
                  <div className="mt-1 text-sm text-text-strong">{String(r['title'])}</div>
                  <div className="mt-1 line-clamp-2 text-xs text-text-muted">{String(r['body'])}</div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Surface>
    </div>
  );
}
