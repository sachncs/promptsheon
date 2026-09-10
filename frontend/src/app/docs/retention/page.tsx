import { DocPage, DocNext } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Retention & purge · Promptsheon',
};

export default function RetentionDoc() {
  return (
    <DocPage
      title="Retention & purge"
      subtitle="Per-org retention. The audit chain is never swept."
    >
      <h2>Default behaviour</h2>
      <ul>
        <li><strong>Default</strong>: 90 days, configurable per org at <code>org.retention.days.&lt;orgId&gt;</code>.</li>
        <li><strong>Frequency</strong>: sweep runs on server start and once every six hours.</li>
        <li><strong>Targets</strong>: <code>eval_results</code> + <code>human_review_queue</code>.</li>
        <li><strong>Excluded</strong>: audit chain (append-only, hash-linked), signature records.</li>
      </ul>

      <h2>Endpoints (admin)</h2>
      <ul>
        <li><code>GET /api/orgs/:id/retention</code> — read current.</li>
        <li><code>PUT /api/orgs/:id/retention</code> — set retention days.</li>
        <li><code>POST /api/orgs/:id/retention/sweep</code> — trigger immediately.</li>
      </ul>

      <h2>Audit</h2>
      <p>Each sweep logs a single <code>org.retention.swept</code> entry to the audit chain with the
        per-table deleted-row counts and the cutoff timestamps.</p>

      <DocNext href="/docs/vault" label="Vault &amp; secret manager" />
    </DocPage>
  );
}
