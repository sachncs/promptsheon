import { DocPage, DocNext, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Quickstart · Promptsheon',
};

export default function Quickstart() {
  return (
    <DocPage
      title="Quickstart"
      subtitle="Stand up a workspace in under sixty seconds, then create your first capability."
    >
      <h2>1. Run the server and reach the dashboard</h2>
      <DocCurl cmd="pnpm install && pnpm dev" />
      <p>The control plane is at <code>http://localhost:3000/app</code>. OpenAPI is at <code>/api/openapi.json</code>.</p>

      <h2>2. Create the admin and org</h2>
      <DocCurl
        cmd={`curl -X POST http://127.0.0.1:8080/api/bootstrap/admin \\
  -H 'content-type: application/json' \\
  -d '{"adminName":"Ada","adminEmail":"ada@example.com","orgName":"Acme AI"}'`}
      />

      <h2>3. Connect a model provider</h2>
      <DocCurl
        cmd={`curl -X POST http://127.0.0.1:8080/api/bootstrap/llm \\
  -H 'content-type: application/json' \\
  -d '{"provider":"openai","model":"gpt-4o-mini","apiKey":"sk-…"}'`}
      />
      <p>The key is stored in the <code>vault_secrets</code> table; the service restarts pick it up via <code>SettingsResolver</code>.</p>

      <h2>4. Author your first capability</h2>
      <p>
        Open <code>/app/editor</code> (the DAG editor), drag agents onto the canvas, wire edges, and
        press Save. The compiler produces a content-addressed manifest. From there, open a merge
        request, get a second reviewer to approve, and merge into <code>main</code>.
      </p>

      <h2>5. Run the eval gate</h2>
      <DocCurl
        cmd={`curl -X POST http://127.0.0.1:8080/api/eval-suites \\
  -H 'X-User-Id: <uid>' -H 'X-Org-Id: <org>' \\
  -d '{"capabilityId":"<cap>","name":"smoke","initialGraders":[{"name":"match","kind":"regex_match","weight":1,"config":{"pattern":"hello","field":"output"}}]}'`}
      />
      <p>Returns <code>{`{ suite, version: { id } }`}</code>. Promote to <code>active</code> through the release workflow.</p>

      <DocNext href="/docs/repos" label="Repositories &amp; the multi-agent DAG" />
    </DocPage>
  );
}
