import { DocPage, DocNext } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Signing · Promptsheon',
};

export default function SigningDoc() {
  return (
    <DocPage
      title="Operator signing keys"
      subtitle="ed25519 detached signatures produced off-box. Anchor trust on the operator's HSM."
    >
      <h2>Why operator-managed keys?</h2>
      <p>Provenance is meaningful only when the private key never leaves the operator's machine. Promptsheon
        records the public half and the detached signature; the platform never sees the secret.</p>

      <h2>Workflow</h2>
      <ol>
        <li>Operator generates an ed25519 keypair locally.</li>
        <li><code>POST /api/orgs/:id/signing-keys</code> with the PEM public key. Fingerprint is computed as sha256(SPKI DER).</li>
        <li>After a merge, the operator signs <code>sha256(oid | ref | approver | timestamp)</code> with their private key.</li>
        <li><code>POST /api/commits/:oid/sign</code> with the signature.</li>
        <li><code>GET /api/commits/:oid/verify</code> re-derives and checks.</li>
      </ol>

      <h2>Rotation</h2>
      <p>Old signatures remain verifiable across keys (the key fingerprint is part of the commit row).
        Revoke a key to disallow signing of new commits while preserving historical verification.</p>

      <DocNext href="/docs/api" label="API reference" />
    </DocPage>
  );
}
