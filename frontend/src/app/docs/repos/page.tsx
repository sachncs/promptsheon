import { DocPage, DocNext, DocCurl, DocHashSample } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Repositories · Promptsheon',
};

export default function ReposPage() {
  return (
    <DocPage
      title="Repositories"
      subtitle="One repository per workspace. Each holds files, branches, tags, commits, and merge requests — content-addressed throughout."
    >
      <h2>Concepts</h2>
      <ul>
        <li><strong>Branch</strong> — a movable pointer to a head commit.</li>
        <li><strong>Tag</strong> — a frozen pointer. Releases are tagged.</li>
        <li><strong>Commit</strong> — content-addressed; oid = sha256(tree + parents + author + timestamp + message).</li>
        <li><strong>Merge request</strong> — gated on author ≠ approver and a per-repo minimum approver count.</li>
        <li><strong>Compile</strong> — walks the tree to produce an immutable manifest hash.</li>
      </ul>

      <h2>Open vs. closed develop</h2>
      <p>Operators sign commits with ed25519 keys uploaded to the organisation. Operators keep the private
        key off-box; the platform records detached signatures attached to commits. The verifier endpoint
        re-derives the payload and checks against the registered public key.</p>
      <DocHashSample />

      <h2>API quick tour</h2>
      <DocCurl cmd="GET /api/repos?workspaceId=…" />
      <DocCurl cmd="POST /api/repos" />
      <DocCurl cmd="POST /api/repos/:id/contents/<path>?ref=main" />
      <DocCurl cmd="POST /api/repos/:id/commits" />
      <DocCurl cmd="POST /api/repos/:id/merge-requests" />
      <DocCurl cmd="POST /api/merge-requests/:id/decisions" />
      <DocCurl cmd="POST /api/merge-requests/:id/merge" />
      <DocCurl cmd="POST /api/commits/:oid/sign" />
      <DocCurl cmd="GET /api/commits/:oid/verify" />

      <DocNext href="/docs/evals" label="Evaluation engine" />
    </DocPage>
  );
}
