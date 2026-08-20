import { DocPage, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'SDK · Promptsheon',
};

export default function SdkDoc() {
  return (
    <DocPage
      title="SDK"
      subtitle="@promptsheon/sdk — typed fetch wrapper over the REST API."
    >
      <h2>Install</h2>
      <DocCurl cmd="import { PromptsheonClient } from '@promptsheon/sdk';" />

      <h2>Construct</h2>
      <DocCurl
        cmd={`const client = new PromptsheonClient({
  baseUrl: 'https://control.example.com',
  apiKey: process.env.PROMPTSHEON_API_KEY!,
});`}
      />

      <h2>Repos</h2>
      <DocCurl cmd="await client.listRepos(workspaceId);" />
      <DocCurl cmd="await client.createRepo({ workspaceId, name });" />
      <DocCurl cmd="await client.putFile(repoId, 'prompts/main.md', '...');" />
      <DocCurl cmd="await client.commit(repoId, 'main', 'init');" />

      <h2>Signing</h2>
      <DocCurl cmd="await client.uploadSigningKey(orgId, label, pem);" />
      <DocCurl cmd="await client.signCommit(oid, keyId, signature);" />
      <DocCurl cmd="await client.verifyCommit(oid);" />

      <h2>Eval</h2>
      <DocCurl cmd="await client.createSuite({ capabilityId, name, ... });" />
      <DocCurl cmd="await client.runSuite(suiteId, { trials });" />
      <DocCurl cmd="await client.evalGate(repoId, [{...}]);" />
    </DocPage>
  );
}
