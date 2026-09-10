import { DocPage, DocNext, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Release workflow · Promptsheon',
};

export default function ReleasesDoc() {
  return (
    <DocPage
      title="Release workflow"
      subtitle="Draft → review → approved → canary → active → rolled back. Maker-checker enforcement. Env overlays. Signed releases."
    >
      <h2>State machine</h2>
      <DocCurl
        cmd={`draft          -> open a release
review         -> submit; non-author approves
approved       -> approve with signature
canary         -> approve posts canary rule
active         -> promote out of canary
rolled_back    -> atomic rollback to a previous release`}
      />

      <h2>Env overlays</h2>
      <p>Per-environment patches merged over the base manifest at build, eval, and execution time:</p>
      <DocCurl cmd="PUT  /api/releases/:id/overlay?environment=staging" />
      <DocCurl cmd="GET /api/releases/:id/overlay?environment=staging" />

      <h2>Canary rules</h2>
      <p>Rule-based canary with segment + window seconds:</p>
      <DocCurl
        cmd={`PUT /api/releases/:id/canary-rule { percent:10, segmentExpr:'tenant == "canary"', windowSeconds:300 }`}
      />

      <h2>Signed releases</h2>
      <p>Operator uploads an ed25519 public key. Signing uses the canonic private key off-box; the platform
        stores the detached signature on the merged commit. Verifier re-derives the payload and checks the
        signature.</p>
      <DocCurl cmd="GET /api/commits/:oid/verify" />

      <DocNext href="/docs/retention" label="Retention &amp; purge" />
    </DocPage>
  );
}
