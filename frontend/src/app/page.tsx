import Link from 'next/link';
import {
  ArrowRight, Boxes, FlaskConical, GitBranch, Workflow, Lock, ShieldCheck,
  Layers, Activity, ScrollText, Terminal,
} from 'lucide-react';
import { Logo } from '@/brand/logo';
import { LogoMark } from '@/brand/logo-mark';
import { Button } from '@/components/ui/button';
import { HashChip } from '@/components/brand/hash-chip';
import { TopNav } from '@/components/brand/top-nav';

const topLinks = [
  { label: 'Product', href: '/#product' },
  { label: 'Workflow', href: '/#workflow' },
  { label: 'Governance', href: '/#governance' },
  { label: 'Docs', href: '/#docs' },
];

function Container({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <div className="mx-auto w-full max-w-6xl px-6">{children}</div>
    </div>
  );
}

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-surface-0 text-foreground">
      <TopNav links={topLinks} />

      {/* Hero */}
      <section className="relative overflow-hidden ps-vignette">
        <Container className="relative pt-24 pb-28">
          <div className="ps-grid-bg absolute inset-0 opacity-30 [mask-image:radial-gradient(ellipse_at_top,black,transparent_60%)]" />
          <div className="relative grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
            <div className="lg:col-span-7">
              <div className="inline-flex items-center gap-2 rounded-full border border-border-subtle bg-surface-1 px-3 py-1 text-xs text-text-muted">
                <span className="h-1.5 w-1.5 rounded-full bg-success" />
                v0.1 — Self-host. No telemetry. Apache-2.0.
              </div>
              <h1 className="mt-6 font-semibold text-h1 md:text-display">
                The control plane for<br />
                <span className="ps-gradient-text">AI capabilities.</span>
              </h1>
              <p className="mt-6 max-w-2xl text-lg leading-relaxed text-text-muted">
                Promptsheon manages prompts, agents, policies, tools, MCP servers,
                guardrails, and evaluation suites as content-addressed, governed,
                version-controlled release artifacts. Not chat. Not notebooks.
                Production infrastructure.
              </p>
              <div className="mt-8 flex flex-wrap items-center gap-3">
                <Link href="/onboarding">
                  <Button size="lg">
                    Open dashboard
                    <ArrowRight className="ml-1.5 h-4 w-4" />
                  </Button>
                </Link>
                <Link href="/#docs">
                  <Button size="lg" variant="outline">
                    Read the docs
                  </Button>
                </Link>
              </div>
              <div className="mt-10 flex items-center gap-6 text-xs text-text-subtle">
                <div className="flex items-center gap-1.5">
                  <Lock className="h-3 w-3" /> On-prem or single-tenant
                </div>
                <div className="flex items-center gap-1.5">
                  <ShieldCheck className="h-3 w-3" /> Maker-checker approvals
                </div>
                <div className="flex items-center gap-1.5">
                  <ScrollText className="h-3 w-3" /> Hash-linked audit chain
                </div>
              </div>
            </div>

            {/* Manifest diff hero */}
            <div className="lg:col-span-5">
              <div className="rounded-2xl border border-border-subtle bg-surface-1 p-5 shadow-2">
                <div className="flex items-center justify-between border-b border-border-subtle pb-3">
                  <div className="flex items-center gap-2 text-xs text-text-subtle">
                    <Terminal className="h-3.5 w-3.5" />
                    <span>capabilities/refund-triage@v3.yaml</span>
                  </div>
                  <HashChip hash="sha256:9c4f…a02b" />
                </div>
                <pre className="mt-4 overflow-x-auto rounded-md bg-surface-0 p-4 text-[12.5px] leading-relaxed font-mono">
                  <code>
                    <span className="text-text-subtle"># capability:</span>{'\n'}
                    <span className="text-brand-highlight">name:</span> refund-triage{'\n'}
                    <span className="text-brand-highlight">version:</span> 3{'\n'}
                    <span className="text-brand-highlight">graph:</span>{'\n'}
                    {'  '}- <span className="text-info">classify</span>: {'{ model: claude-3-5 }'}{'\n'}
                    {'  '}- <span className="text-info">retrieve</span>: {'{ tool: orders }'}{'\n'}
                    {'  '}- <span className="text-info">decide</span>:{' '}
                    <span className="text-warning">~ reviewer policy v2 →</span>{'\n'}
                    {'  '}- <span className="text-info">respond</span>: {'{ guardrail: pii-redact }'}{'\n'}
                    {'\n'}
                    <span className="text-text-subtle"># eval:</span>{'\n'}
                    <span className="text-brand-highlight">suite:</span> refund-suite{'\n'}
                    <span className="text-brand-highlight">threshold:</span>{' '}
                    <span className="text-success">0.92</span>{'\n'}
                  </code>
                </pre>
                <div className="mt-3 flex items-center justify-between text-xs text-text-subtle">
                  <div className="flex items-center gap-3">
                    <span><span className="text-warning">~ reviewer policy v2</span> +12 lines</span>
                    <span className="text-destructive">- pii-redact scrub list</span>
                  </div>
                  <span className="text-success">score 0.94 / 0.92</span>
                </div>
              </div>
            </div>
          </div>
        </Container>
      </section>

      {/* Pillars */}
      <section id="product" className="border-t border-border-subtle bg-surface-1/40">
        <Container className="py-20">
          <div className="text-center">
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
              Three pillars
            </div>
            <h2 className="mt-3 font-semibold text-h2 md:text-h1 text-text-strong">
              Treat AI capabilities like software.
            </h2>
            <p className="mt-4 text-text-muted max-w-2xl mx-auto text-base leading-relaxed">
              The same practices you expect from infrastructure: addressable artifacts,
              reproducible builds, gated releases, and an audit chain.
            </p>
          </div>
          <div className="mt-12 grid gap-5 md:grid-cols-3">
            {[
              {
                icon: Boxes,
                title: 'Content-addressed registry',
                text: 'Every manifest, prompt block, tool spec, and policy snapshot is hashed and stored by content. Same input, same hash, every time.',
                detail: 'manifest CAS',
              },
              {
                icon: FlaskConical,
                title: 'Evaluation engine',
                text: 'Run datasets, score with deterministic or LLM-judge scorers, gate releases on thresholds, detect regressions, compare models.',
                detail: 'eval passes',
              },
              {
                icon: GitBranch,
                title: 'Governed releases',
                text: 'Draft → review → approved → canary → active → rolled back. Every transition is signed, audited, reversible in one click.',
                detail: 'release path',
              },
            ].map(({ icon: Icon, title, text, detail }) => (
              <article
                key={title}
                className="group rounded-2xl border border-border-subtle bg-surface-1 p-6 transition-colors hover:border-border-strong"
              >
                <div className="flex items-center justify-between">
                  <Icon className="h-5 w-5 text-brand-highlight" />
                  <span className="rounded-full border border-border-subtle bg-surface-2 px-2 py-0.5 text-[10px] uppercase tracking-wider text-text-subtle">
                    {detail}
                  </span>
                </div>
                <h3 className="mt-5 font-semibold text-h4 text-text-strong">{title}</h3>
                <p className="mt-2 text-sm text-text-muted leading-relaxed">{text}</p>
              </article>
            ))}
          </div>
        </Container>
      </section>

      {/* Workflow */}
      <section id="workflow" className="border-t border-border-subtle">
        <Container className="py-20">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12">
            <div className="lg:col-span-4">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
                Workflow
              </div>
              <h2 className="mt-3 font-semibold text-h2 md:text-h1 text-text-strong">
                From intent to release.
              </h2>
              <p className="mt-4 text-text-muted text-base leading-relaxed">
                Capabilities move through a deterministic state machine. Every transition
                records who acted and on what evidence.
              </p>
            </div>
            <ol className="lg:col-span-8 space-y-4">
              {[
                { step: '01', title: 'Author', body: 'Describe the capability. The planner decomposes it into a DAG of agents, policies, and tools.' },
                { step: '02', title: 'Compile', body: 'The compiler produces an immutable manifest. Its content hash is its identity.' },
                { step: '03', title: 'Evaluate', body: 'Run suites. Gate on threshold. Block merges on regressions.' },
                { step: '04', title: 'Approve', body: 'A second pair of eyes signs off. The creator cannot approve their own release.' },
                { step: '05', title: 'Canary → Active', body: 'Weighted rollout. Live eval scores watch for drift. One-click rollback.' },
              ].map((s) => (
                <li key={s.step} className="flex gap-5 rounded-xl border border-border-subtle bg-surface-1 p-5">
                  <div className="shrink-0 grid h-10 w-10 place-items-center rounded-lg border border-border-subtle bg-surface-2 font-mono text-xs text-text-muted">
                    {s.step}
                  </div>
                  <div>
                    <div className="font-semibold text-text-strong">{s.title}</div>
                    <div className="mt-1 text-sm text-text-muted">{s.body}</div>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </Container>
      </section>

      {/* Trust / Governance */}
      <section id="governance" className="border-t border-border-subtle bg-surface-1/40">
        <Container className="py-20">
          <div className="grid gap-10 md:grid-cols-2">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
                Trust & governance
              </div>
              <h2 className="mt-3 font-semibold text-h2 text-text-strong">
                Built for procurement, security, and risk.
              </h2>
              <p className="mt-4 text-text-muted text-base leading-relaxed">
                Every capability has a reproducible history. Every release has an
                approver. Every audit row is hash-linked.
              </p>
            </div>
            <div className="grid sm:grid-cols-2 gap-3">
              {[
                { icon: Lock, title: 'Content-addressed', text: 'Same input, same artifact hash, same provenance.' },
                { icon: ShieldCheck, title: 'Maker-checker', text: 'Approval workflow; creators cannot self-approve.' },
                { icon: ScrollText, title: 'Hash-linked audit', text: 'Append-only chain. Verification endpoint in product.' },
                { icon: Layers, title: 'Rollback', text: 'Revert any release in one click. State is preserved.' },
              ].map(({ icon: Icon, title, text }) => (
                <div key={title} className="rounded-xl border border-border-subtle bg-surface-1 p-4">
                  <Icon className="h-4 w-4 text-brand-highlight" />
                  <div className="mt-2 text-sm font-semibold text-text-strong">{title}</div>
                  <div className="mt-0.5 text-xs text-text-muted">{text}</div>
                </div>
              ))}
            </div>
          </div>
        </Container>
      </section>

      {/* CTA */}
      <section className="border-t border-border-subtle bg-surface-0">
        <Container className="py-20 text-center">
          <LogoMark size={48} className="mx-auto" />
          <h2 className="mt-6 font-semibold text-h2 md:text-h1 text-text-strong">
            Ship AI capabilities, not prompts.
          </h2>
          <p className="mt-4 text-text-muted max-w-xl mx-auto text-base leading-relaxed">
            Set up Promptsheon on your own infrastructure. Configure your provider,
            create your first capability, route a release.
          </p>
          <div className="mt-8 flex items-center justify-center gap-3">
            <Link href="/onboarding">
              <Button size="lg">Start setup</Button>
            </Link>
            <Link href="/#docs">
              <Button size="lg" variant="outline">Read documentation</Button>
            </Link>
          </div>
        </Container>
      </section>

      {/* Footer */}
      <footer className="border-t border-border-subtle bg-surface-1/50">
        <Container className="py-10">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-6">
            <div className="flex items-center gap-4">
              <Logo size="xs" showWordmark />
              <span className="text-xs text-text-subtle">Apache-2.0</span>
            </div>
            <div className="flex gap-6 text-xs text-text-muted">
              <Link href="/onboarding">Setup</Link>
              <a href="https://github.com/sachncs/promptsheon" className="hover:text-text-default">GitHub</a>
              <Link href="/#docs" className="hover:text-text-default">Docs</Link>
              <Link href="/#governance" className="hover:text-text-default">Security</Link>
            </div>
          </div>
          <div className="mt-6 text-[11px] text-text-subtle">
            Promptsheon is self-hosted infrastructure. By using the software you
            accept responsibility for the content your capabilities produce.
          </div>
        </Container>
      </footer>
    </div>
  );
}
