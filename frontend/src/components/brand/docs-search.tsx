'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Input } from '@/components/ui/input';

const ALL_LINKS: Array<{ href: string; label: string; section: string }> = [
  { href: '/docs/quickstart', label: 'Set up your workspace', section: 'Quickstart' },
  { href: '/docs/onboarding', label: 'Connect a model provider', section: 'Quickstart' },
  { href: '/docs/repos', label: 'Repositories', section: 'Capabilities' },
  { href: '/docs/dag', label: 'Multi-agent DAG', section: 'Capabilities' },
  { href: '/docs/releases', label: 'Release workflow', section: 'Capabilities' },
  { href: '/docs/evals', label: 'Evaluation engine', section: 'Quality' },
  { href: '/docs/grading', label: 'Graders', section: 'Quality' },
  { href: '/docs/calibration', label: 'Human review & calibration', section: 'Quality' },
  { href: '/docs/vault', label: 'Vault & secret manager', section: 'Security' },
  { href: '/docs/signing', label: 'Operator signing keys', section: 'Security' },
  { href: '/docs/retention', label: 'Retention & purge', section: 'Platform' },
  { href: '/docs/api', label: 'HTTP API reference', section: 'Platform' },
  { href: '/docs/cli', label: 'CLI', section: 'Platform' },
  { href: '/docs/sdk', label: 'SDK', section: 'Platform' },
];

export function DocsSearch() {
  const [search, setSearch] = useState('');
  const filtered = search
    ? ALL_LINKS.filter((l) => l.label.toLowerCase().includes(search.toLowerCase()))
    : [];

  return (
    <div>
      <Input
        placeholder="Search docs…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="text-sm"
      />
      {search && (
        <ul className="mt-3 space-y-1.5">
          {filtered.length === 0 ? (
            <li className="text-xs text-text-muted">No matches.</li>
          ) : filtered.map((l) => (
            <li key={l.href}>
              <Link href={l.href} className="block rounded-md px-2 py-1 text-sm text-text-default hover:bg-surface-2">
                <span className="font-medium">{l.label}</span>
                <span className="block text-[11px] uppercase tracking-wider text-text-subtle">{l.section}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}