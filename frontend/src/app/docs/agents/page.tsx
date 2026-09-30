import Link from 'next/link';
import { DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Promptsheon · Agent specifications',
  description: 'Build, validate, publish, execute, and inspect immutable agent specifications.',
};

export default function AgentSpecificationsDocsPage() {
  return (
    <article className="prose prose-invert max-w-none space-y-10">
      <header>
        <div className="text-micro font-semibold uppercase tracking-[0.16em] text-text-subtle">Capabilities / Agents</div>
        <h1 className="mt-3 font-semibold text-h1 text-text-strong">Build an agent specification</h1>
        <p className="mt-4 max-w-3xl text-base leading-relaxed text-text-muted">
          Promptsheon stores every agent as an immutable, content-addressed executable specification.
          A revision combines the role, objective, prompt, policies, tools, permissions, budgets,
          evaluation intent, and lifecycle controls needed to reproduce an execution.
        </p>
      </header>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">The development loop</h2>
        <div className="mt-4 rounded-xl border border-border-subtle bg-surface-2/40 p-5 font-mono text-sm leading-8 text-text-default">
          Build → Validate → Create draft → Review diff → Publish → Execute → Observe evidence
        </div>
        <p className="mt-3 text-sm leading-relaxed text-text-muted">
          The local simulator is the default development path. It exercises validation, durable
          execution, budgets, cancellation, and evidence persistence without an external model key.
        </p>
      </section>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">Minimum specification</h2>
        <pre className="mt-4 overflow-x-auto rounded-xl border border-border-subtle bg-surface-0 p-5 font-mono text-xs leading-relaxed text-text-muted">{`{
  "role": "Research assistant",
  "objective": "Answer questions with evidence.",
  "prompt": { "system": "Be precise." },
  "modelPolicy": {
    "provider": "simulator",
    "model": "promptsheon-simulator"
  },
  "lifecycle": { "owner": "team-research" }
}`}</pre>
        <p className="mt-3 text-sm leading-relaxed text-text-muted">The shared schema supplies safe defaults for optional policies. Use the validation endpoint before persisting a revision.</p>
      </section>

      <section className="space-y-5">
        <h2 className="font-semibold text-h2 text-text-strong">Validate and create</h2>
        <DocCurl cmd="POST /api/workspaces/{workspaceId}/agent-specifications/validate" />
        <DocCurl cmd="POST /api/workspaces/{workspaceId}/agent-specifications" />
        <p className="text-sm leading-relaxed text-text-muted">Creation returns the SHA-256 revision hash. The content is stored in content-addressed storage and metadata is tenant-scoped in SQLite.</p>
      </section>

      <section className="space-y-5">
        <h2 className="font-semibold text-h2 text-text-strong">Review and publish</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          {[
            ['Inspect', 'GET /api/workspaces/{workspaceId}/agent-specifications/{hash}'],
            ['Diff', 'POST /api/workspaces/{workspaceId}/agent-specifications/diff'],
            ['Publish', 'POST /api/workspaces/{workspaceId}/agent-specifications/{hash}/publish'],
          ].map(([label, path]) => <div key={label} className="rounded-lg border border-border-subtle bg-surface-2/40 p-4"><div className="text-xs font-semibold uppercase tracking-wider text-text-subtle">{label}</div><code className="mt-2 block break-all text-xs text-text-muted">{path}</code></div>)}
        </div>
        <p className="text-sm leading-relaxed text-text-muted">Published revisions remain immutable. Change a policy by creating a child revision with <code>parentHash</code>, then review the generated diff.</p>
      </section>

      <section className="space-y-5">
        <h2 className="font-semibold text-h2 text-text-strong">Execute without a provider key</h2>
        <DocCurl cmd="POST /api/workspaces/{workspaceId}/execution-jobs" />
        <pre className="overflow-x-auto rounded-xl border border-border-subtle bg-surface-0 p-5 font-mono text-xs leading-relaxed text-text-muted">{`{
  "agentHash": "<64-character revision hash>",
  "inputs": { "input": "hello" },
  "idempotencyKey": "local-run-001"
}`}</pre>
        <p className="text-sm leading-relaxed text-text-muted">Poll the returned job with <code>GET /api/workspaces/{'{workspaceId}'}/execution-jobs/{'{id}'}</code>. Queued and running jobs can be cancelled. Every attempt records redacted evidence keyed by the agent hash.</p>
      </section>

      <section>
        <h2 className="font-semibold text-h2 text-text-strong">Continue building</h2>
        <p className="mt-3 text-sm leading-relaxed text-text-muted">Use the <Link href="/docs/cli" className="text-text-strong underline-offset-4 hover:underline">CLI</Link> for automation, the <Link href="/docs/sdk" className="text-text-strong underline-offset-4 hover:underline">TypeScript SDK</Link> for services, or open the <Link href="/app/agents" className="text-text-strong underline-offset-4 hover:underline">agent console</Link> for an interactive workflow.</p>
      </section>
    </article>
  );
}
