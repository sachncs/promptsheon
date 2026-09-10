import Link from 'next/link';
import { Github } from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Surface } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { Breadcrumb } from '@/components/brand/breadcrumb';
import { DocsSearch } from '@/components/brand/docs-search';

const sections = [
  {
    title: 'Quickstart',
    links: [
      { href: '/docs/quickstart', label: 'Set up your workspace' },
      { href: '/docs/onboarding', label: 'Connect a model provider' },
    ],
  },
  {
    title: 'Capabilities',
    links: [
      { href: '/docs/repos', label: 'Repositories' },
      { href: '/docs/dag', label: 'Multi-agent DAG' },
      { href: '/docs/releases', label: 'Release workflow' },
    ],
  },
  {
    title: 'Quality',
    links: [
      { href: '/docs/evals', label: 'Evaluation engine' },
      { href: '/docs/grading', label: 'Graders' },
      { href: '/docs/calibration', label: 'Human review & calibration' },
    ],
  },
  {
    title: 'Security',
    links: [
      { href: '/docs/vault', label: 'Vault & secret manager' },
      { href: '/docs/signing', label: 'Operator signing keys' },
    ],
  },
  {
    title: 'Platform',
    links: [
      { href: '/docs/retention', label: 'Retention & purge' },
      { href: '/docs/api', label: 'HTTP API reference' },
      { href: '/docs/cli', label: 'CLI' },
      { href: '/docs/sdk', label: 'SDK' },
    ],
  },
];

export const metadata = {
  title: 'Promptsheon · Docs',
  description: 'Git-native, content-addressed control plane for AI capabilities.',
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-surface-0">
      <header className="sticky top-0 z-40 border-b border-border-subtle bg-surface-0/85 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-6 px-6">
          <Link href="/docs">
            <Logo size="sm" />
          </Link>
          <nav className="hidden gap-1 sm:flex">
            {sections.map((s) => (
              <Link
                key={s.title}
                href={`/docs#${s.title.toLowerCase().replace(/ /g, '-')}`}
                className="rounded-md px-3 py-1.5 text-sm text-text-muted hover:bg-surface-2 hover:text-text-default"
              >
                {s.title}
              </Link>
            ))}
          </nav>
          <div className="ml-auto flex items-center gap-2">
            <a
              href="https://github.com/sachncs/promptsheon"
              target="_blank"
              rel="noreferrer"
              className="hidden sm:inline-flex h-9 w-9 items-center justify-center rounded-md text-text-muted hover:bg-surface-2 hover:text-text-default"
              aria-label="GitHub"
            >
              <Github className="h-4 w-4" />
            </a>
            <Link href="/app">
              <Button size="sm">Open dashboard</Button>
            </Link>
          </div>
        </div>
      </header>
      <div className="mx-auto flex w-full max-w-6xl gap-8 px-6 py-10">
        <aside className="hidden w-56 shrink-0 lg:block">
          <div className="sticky top-24 space-y-6">
            <DocsSearch />
            {sections.map((s) => (
              <section key={s.title}>
                <h4 id={s.title.toLowerCase().replace(/ /g, '-')} className="mb-2 text-xs font-semibold uppercase tracking-[0.08em] text-text-subtle">
                  {s.title}
                </h4>
                <ul className="space-y-1.5">
                  {s.links.map((l) => (
                    <li key={l.href}>
                      <Link href={l.href} className="text-sm text-text-muted hover:text-text-default">
                        {l.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        </aside>
        <main className="min-w-0 flex-1 space-y-4">
          <Breadcrumb
            items={[
              { label: 'Docs', href: '/docs' },
              { label: 'Page' },
            ]}
          />
          <Surface>{children}</Surface>
          <div className="flex items-center justify-between text-xs text-text-subtle">
            <a
              href="https://github.com/sachncs/promptsheon/edit/master/frontend/src/app/docs"
              target="_blank"
              rel="noreferrer"
              className="hover:text-text-default"
            >
              Edit this page on GitHub →
            </a>
            <span>Apache-2.0 · Promptsheon self-hosted</span>
          </div>
        </main>
      </div>
    </div>
  );
}