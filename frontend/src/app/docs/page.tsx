import Link from 'next/link';
import { HashChip } from '@/components/brand/hash-chip';
import { Badge } from '@/components/ui/badge';

export default function DocsIndex() {
  return (
    <article className="prose prose-invert max-w-none space-y-10">
      <header>
        <div className="text-micro font-semibold uppercase tracking-[0.16em] text-text-subtle">Promptsheon</div>
        <h1 className="mt-3 font-semibold text-h1 text-text-strong">
          The control plane for AI capabilities.
        </h1>
        <p className="mt-4 max-w-2xl text-text-muted text-base leading-relaxed">
          Git-native version control, content-addressed artifacts, governed releases, evaluation
          gates, audit chain, and an operator-managed signing path — all backed by a single repository
          per workspace. Self-hosted. Apache-2.0.
        </p>
      </header>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">What you get</h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          {[
            { tag: 'Repository', body: 'Files, branches, tags, commits, merge requests, deterministic oid-space. Each capability is a DAG of agents.' },
            { tag: 'Release workflow', body: 'Draft → review → approved → canary → active → rolled back. Env overlays. Maker-checker enforcement.' },
            { tag: 'Evaluation engine', body: 'Versioned suites, regex / schema / tool-call / transcript graders, pass@k and pass^k, human-review queue, calibration report.' },
            { tag: 'Vault + signing', body: 'AES-256-GCM at rest with a swappable KMS, key rotation, ed25519 detached signatures from operator-managed keys.' },
            { tag: 'Operator surface', body: 'OpenAPI 3.1, CLI, TypeScript SDK, /api/openapi.json, tamper-evident export, FTS5 search.' },
            { tag: 'Audit chain', body: 'Hash-linked, append-only, with a /api/audit/verify endpoint and a retention cron that never sweeps the chain.' },
          ].map((b) => (
            <div key={b.tag} className="rounded-xl border border-border-subtle bg-surface-2/40 p-4">
              <Badge className="bg-surface-3">{b.tag}</Badge>
              <p className="mt-3 text-sm text-text-muted">{b.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">A capability is a multi-agent DAG</h2>
        <p className="mt-3 text-text-muted text-base leading-relaxed">
          A capability lives in a repository on a branch. The DAG inside it composes agents, prompts,
          tools, MCP servers, guardrails, evaluation hooks, and memory contracts. Compiling the tree
          yields a content-addressed manifest that drives execution and authoring review.
        </p>
        <div className="mt-4 rounded-xl border border-border-subtle bg-surface-0 p-5">
          <p className="text-sm font-semibold text-text-default">Example layout</p>
          <pre className="mt-3 overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed text-text-muted">
{`repo://acme/refund-triage
├── prompts/
│   └── main.md
├── policies/
│   └── reviewer.json
├── tools/
│   └── orders.yaml
├── mcp/
│   └── stripe.yaml
├── guardrails/
│   └── pii-redact.json
└── eval/
    └── refund-suite.json`}
          </pre>
          <div className="mt-3 flex items-center gap-2 text-xs text-text-subtle">
            <HashChip hash="sha256:9c4f…a02b" length={20} />
            <span>deterministic, content-addressed, addressable via MR</span>
          </div>
        </div>
      </section>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">Next</h2>
        <ul className="mt-3 space-y-2 text-text-muted">
          <li>· <Link href="/docs/quickstart" className="text-text-strong underline-offset-4 hover:underline">Set up your workspace</Link></li>
          <li>· <Link href="/docs/repos" className="text-text-strong underline-offset-4 hover:underline">Repositories &amp; the multi-agent DAG</Link></li>
          <li>· <Link href="/docs/releases" className="text-text-strong underline-offset-4 hover:underline">Release workflow</Link></li>
          <li>· <Link href="/docs/evals" className="text-text-strong underline-offset-4 hover:underline">Evaluation engine</Link></li>
          <li>· <Link href="/docs/api" className="text-text-strong underline-offset-4 hover:underline">API reference</Link></li>
        </ul>
      </section>
    </article>
  );
}
