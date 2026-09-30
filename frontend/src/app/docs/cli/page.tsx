import { DocPage, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'CLI · Promptsheon',
};

export default function CliDoc() {
  return (
    <DocPage
      title="CLI"
      subtitle="promptsheon — subcommand harness for CI scripts and developer ergonomics."
    >
      <h2>Install</h2>
      <DocCurl cmd="pnpm install && pnpm --dir cli build" />
      <p>The CLI is <code>@promptsheon/cli</code> in the monorepo. The binary entrypoint is <code>promptsheon</code>.</p>

      <h2>Configuration</h2>
      <DocCurl cmd="export PROMPTSHEON_API_URL=http://127.0.0.1:8080" />
      <DocCurl cmd="export PROMPTSHEON_API_KEY=pk_…" />
      <DocCurl cmd="export PROMPTSHEON_WORKSPACE_ID=…" />

      <h2>Commands</h2>
      <DocCurl cmd="promptsheon login                       # verify the API key works" />
      <DocCurl cmd="promptsheon repos list                 # list repositories in the workspace" />
      <DocCurl cmd="promptsheon eval gate <repoId>        # CI gate" />
      <DocCurl cmd="promptsheon release approve <id>      # approve a release" />
      <DocCurl cmd="promptsheon evidence list             # newest evidence records" />
      <DocCurl cmd="promptsheon evidence trace <traceId>  # chronological trace evidence" />
      <DocCurl cmd="PROMPTSHEON_PARENT_HASH=<sha256> PROMPTSHEON_CHANGE_REASON='refine prompt' promptsheon spec create agent.json" />
      <p>Use <code>--json</code> for machine-readable output. Evidence is already redacted by the API before it reaches the CLI.</p>
    </DocPage>
  );
}
