import Link from 'next/link';
import { Logo } from '@/brand/logo';
import { Surface } from '@/components/brand/surface';

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
            <Link href="/app" className="rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-sm text-text-default hover:border-border-strong">
              Open dashboard
            </Link>
          </div>
        </div>
      </header>
      <div className="mx-auto flex w-full max-w-6xl gap-8 px-6 py-12">
        <aside className="hidden w-56 shrink-0 lg:block">
          <div className="sticky top-24 space-y-6">
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
        <main className="min-w-0 flex-1">
          <Surface>{children}</Surface>
        </main>
      </div>
      <footer className="border-t border-border-subtle py-8 text-center text-xs text-text-subtle">
        Apache-2.0 · Promptsheon self-hosted
      </footer>
    </div>
  );
}
