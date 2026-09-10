import Link from 'next/link';
import {
  ArrowRight, Boxes, FlaskConical, GitBranch, Workflow, Lock, ShieldCheck,
  Layers, Activity, ScrollText, Terminal, CheckCircle2, Sparkles,
  Compass, Fingerprint, Network, Rocket, Telescope,
} from 'lucide-react';
import { Logo } from '@/brand/logo';
import { LogoMark } from '@/brand/logo-mark';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { HashChip } from '@/components/brand/hash-chip';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
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

      {/* Logo cloud */}
      <section className="border-t border-border-subtle">
        <Container className="py-12">
          <div className="text-center text-[11px] font-semibold uppercase tracking-[0.16em] text-text-subtle">
            Self-hosted by teams running agents in production
          </div>
          <div className="mt-6 grid grid-cols-3 gap-6 sm:grid-cols-5">
            {[':: / /acme', ':: / /northwind', ':: / /lyra', ':: / /octant', ':: / /orbital'].map((name) => (
              <div key={name} className="flex h-12 items-center justify-center rounded-lg border border-border-subtle bg-surface-1 font-mono text-xs text-text-subtle">
                {name}
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Feature grid */}
      <section id="features" className="border-t border-border-subtle bg-surface-1/40">
        <Container className="py-20">
          <div className="text-center">
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
              What you get
            </div>
            <h2 className="mt-3 font-semibold text-h2 md:text-h1 text-text-strong">
              Six capabilities that ship together.
            </h2>
          </div>
          <div className="mt-12 grid gap-5 md:grid-cols-2 lg:grid-cols-3">
            {[
              { icon: Compass, title: 'Visual DAG editor', text: 'Compose agents, policies, tools, and memory into a graph. Live preview against a real execution.' },
              { icon: Network, title: 'Releases with canary', text: 'Weighted traffic split. Live eval scores monitor canary. One-click rollback, hash-linked.' },
              { icon: ShieldCheck, title: 'Maker-checker approvals', text: 'Creator cannot approve their own release. Reason + voter persisted with the audit row.' },
              { icon: Fingerprint, title: 'Content-addressed CAS', text: 'Every compiled manifest is hashed and stored by content. Same input, same hash.' },
              { icon: FlaskConical, title: 'Evaluation engine', text: 'Declarative datasets, pluggable scorers, regression gates, parallel runs.' },
              { icon: Telescope, title: 'Self-evolution loop', text: 'Watches live eval scores; on regression, re-plans and re-releases with cooldown.' },
            ].map(({ icon: Icon, title, text }) => (
              <article
                key={title}
                className="group rounded-2xl border border-border-subtle bg-surface-1 p-6 transition-colors hover:border-border-strong"
              >
                <div className="grid h-10 w-10 place-items-center rounded-lg bg-brand/15 text-brand">
                  <Icon className="h-5 w-5" />
                </div>
                <h3 className="mt-5 font-semibold text-h4 text-text-strong">{title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-text-muted">{text}</p>
              </article>
            ))}
          </div>
        </Container>
      </section>

      {/* Comparison */}
      <section id="compare" className="border-t border-border-subtle">
        <Container className="py-20">
          <div className="text-center">
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
              Comparison
            </div>
            <h2 className="mt-3 font-semibold text-h2 md:text-h1 text-text-strong">
              Not a notebook. Not hosted SaaS.
            </h2>
            <p className="mt-4 max-w-2xl mx-auto text-base text-text-muted leading-relaxed">
              Promptsheon sits between ad-hoc notebooks and hosted agent platforms. You own the infrastructure; we give you the engineering.
            </p>
          </div>
          <div className="mt-10 grid gap-3 md:grid-cols-3">
            {[
              { label: 'Notebooks', tone: 'text-text-subtle', rows: [
                ['Visibility', '—'],
                ['Audit chain', '—'],
                ['Maker-checker', '—'],
                ['Canary rollout', '—'],
              ] },
              { label: 'Hosted SaaS', tone: 'text-text-subtle', rows: [
                ['Visibility', 'partial'],
                ['Audit chain', 'provider-side'],
                ['Maker-checker', 'optional'],
                ['Canary rollout', 'beta'],
              ] },
              { label: 'Promptsheon', tone: 'text-brand', rows: [
                ['Visibility', 'self-hosted, full'],
                ['Audit chain', 'hash-linked, verifiable'],
                ['Maker-checker', 'enforced'],
                ['Canary rollout', 'first-class'],
              ], highlight: true },
            ].map((col) => (
              <div
                key={col.label}
                className={`rounded-2xl border p-5 ${col.highlight ? 'border-brand bg-brand/5 shadow-glow' : 'border-border-subtle bg-surface-1'}`}
              >
                <div className={`text-sm font-semibold ${col.tone}`}>{col.label}</div>
                <dl className="mt-3 space-y-2">
                  {col.rows.map(([k, v]) => (
                    <div key={k} className="flex items-center justify-between text-sm">
                      <dt className="text-text-muted">{k}</dt>
                      <dd className={col.highlight ? 'text-text-strong font-medium' : 'text-text-muted'}>{v}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Pricing */}
      <section id="pricing" className="border-t border-border-subtle bg-surface-1/40">
        <Container className="py-20">
          <div className="text-center">
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">
              Pricing
            </div>
            <h2 className="mt-3 font-semibold text-h2 md:text-h1 text-text-strong">
              One tier. Self-hosted. Apache-2.0.
            </h2>
            <p className="mt-4 max-w-2xl mx-auto text-base text-text-muted leading-relaxed">
              No per-seat pricing, no cloud bill. Run it on your own metal, behind your own firewall.
            </p>
          </div>
          <div className="mt-12 grid gap-5 md:grid-cols-3">
            <Surface>
              <SurfaceHeader title="Self-host" />
              <div className="text-3xl font-semibold text-text-strong">Free</div>
              <p className="mt-1 text-sm text-text-muted">Apache-2.0. Run anywhere you can run Node.</p>
              <ul className="mt-5 space-y-2 text-sm">
                {['All 76 REST endpoints', 'DAG editor, eval engine, audit chain', 'Multi-provider LLM', 'No telemetry'].map((f) => (
                  <li key={f} className="flex items-start gap-2">
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-success" />
                    <span className="text-text-default">{f}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-6">
                <Link href="/onboarding"><Button className="w-full">Get started</Button></Link>
              </div>
            </Surface>
            <Surface>
              <SurfaceHeader
                title="Cloud (soon)"
                actions={<Badge>Waitlist</Badge>}
              />
              <div className="text-3xl font-semibold text-text-strong">TBD</div>
              <p className="mt-1 text-sm text-text-muted">Hosted Promptsheon with managed upgrades.</p>
              <ul className="mt-5 space-y-2 text-sm">
                {['Same Apache-2.0 codebase', 'Zero-ops upgrade path', 'Backups + observability included', 'Region-pinned'].map((f) => (
                  <li key={f} className="flex items-start gap-2">
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-text-muted" />
                    <span className="text-text-muted">{f}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-6">
                <Button variant="outline" className="w-full" disabled>Join waitlist</Button>
              </div>
            </Surface>
            <Surface>
              <SurfaceHeader title="Enterprise" />
              <div className="text-3xl font-semibold text-text-strong">Talk to us</div>
              <p className="mt-1 text-sm text-text-muted">Air-gapped installs, SSO, custom integrations.</p>
              <ul className="mt-5 space-y-2 text-sm">
                {['On-prem deployment playbook', 'SSO via your IdP', 'Custom audit retention', 'Engineering contact'].map((f) => (
                  <li key={f} className="flex items-start gap-2">
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-text-muted" />
                    <span className="text-text-muted">{f}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-6">
                <a href="mailto:sachncs@gmail.com"><Button variant="outline" className="w-full">Email us</Button></a>
              </div>
            </Surface>
          </div>
        </Container>
      </section>

      {/* Footer */}
      <footer className="border-t border-border-subtle bg-surface-1/50">
        <Container className="py-12">
          <div className="grid gap-10 md:grid-cols-5">
            <div className="md:col-span-2">
              <Logo size="sm" showWordmark />
              <p className="mt-3 max-w-xs text-sm text-text-muted">
                The control plane for AI capabilities. Self-hosted, content-addressed, governed.
              </p>
              <div className="mt-4 flex items-center gap-2">
                <Badge>Apache-2.0</Badge>
                <Badge>Self-hosted</Badge>
                <Badge>No telemetry</Badge>
              </div>
            </div>
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-text-subtle">Product</div>
              <ul className="mt-3 space-y-2 text-sm text-text-muted">
                <li><Link href="/#product" className="hover:text-text-default">Capabilities</Link></li>
                <li><Link href="/#features" className="hover:text-text-default">Features</Link></li>
                <li><Link href="/#compare" className="hover:text-text-default">Compare</Link></li>
                <li><Link href="/#pricing" className="hover:text-text-default">Pricing</Link></li>
              </ul>
            </div>
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-text-subtle">Resources</div>
              <ul className="mt-3 space-y-2 text-sm text-text-muted">
                <li><Link href="/docs" className="hover:text-text-default">Documentation</Link></li>
                <li><Link href="/docs/quickstart" className="hover:text-text-default">Quickstart</Link></li>
                <li><a href="https://github.com/sachncs/promptsheon" className="hover:text-text-default">GitHub</a></li>
                <li><a href="https://github.com/sachncs/promptsheon/issues/new" className="hover:text-text-default">Issues</a></li>
              </ul>
            </div>
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-text-subtle">Company</div>
              <ul className="mt-3 space-y-2 text-sm text-text-muted">
                <li><Link href="/#governance" className="hover:text-text-default">Security</Link></li>
                <li><a href="mailto:sachncs@gmail.com" className="hover:text-text-default">Contact</a></li>
                <li><a href="LICENSE" className="hover:text-text-default">License</a></li>
              </ul>
            </div>
          </div>
          <div className="mt-10 flex flex-wrap items-center justify-between gap-4 border-t border-border-subtle pt-6 text-xs text-text-subtle">
            <span>© 2026 Sachin · Apache-2.0</span>
            <span>Promptsheon is self-hosted infrastructure. By using the software you accept responsibility for the content your capabilities produce.</span>
          </div>
        </Container>
      </footer>
    </div>
  );
}
