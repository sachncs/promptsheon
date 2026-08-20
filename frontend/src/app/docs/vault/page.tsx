import { DocPage, DocNext, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Vault & secret manager · Promptsheon',
};

export default function VaultDoc() {
  return (
    <DocPage
      title="Vault & secret manager"
      subtitle="AES-256-GCM at rest with a swappable KMS. Single-row keyring with rotation."
    >
      <h2>Storage shape</h2>
      <p>Every secret lives in <code>vault_secrets</code>; every key version lives in <code>vault_keyring</code>.
        A partial unique index enforces exactly one active key.</p>

      <h2>Operator surface</h2>
      <DocCurl cmd="POST /api/orgs/:id/signing-keys   (PEM/SPKI ED25519)" />
      <DocCurl cmd="GET  /api/vault/keys              (keyring)" />
      <DocCurl cmd="POST /api/vault/keys/rotate      (mint + re-encrypt)" />
      <DocCurl cmd="POST /api/vault/secrets         (write)" />
      <DocCurl cmd="GET  /api/vault/secrets          (list)" />

      <h2>Swapping to a real KMS</h2>
      <p>The <code>Kms</code> interface is two methods: <code>resolve(fingerprint)</code> and
        <code>generate(label)</code>. Production deployments ship <code>AwsSecretsManagerKms</code>,
        <code>VaultKms</code>, or <code>DopplerKms</code> by satisfying the interface — the rest of
        the vault never touches the cipher directly.</p>

      <DocNext href="/docs/signing" label="Operator signing keys" />
    </DocPage>
  );
}
