import { DocPage, DocNext, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'API · Promptsheon',
};

export default function ApiDoc() {
  return (
    <DocPage
      title="API reference"
      subtitle="OpenAPI 3.1 document at /api/openapi.json. Bearer auth. X-User-Id + X-Org-Id internal fallback for dev tooling."
    >
      <h2>Download</h2>
      <DocCurl cmd="curl http://127.0.0.1:8080/api/openapi.json | jq" />

      <h2>Public endpoints</h2>
      <DocCurl cmd="GET    /api/health" />
      <DocCurl cmd="GET    /api/openapi.json" />
      <DocCurl cmd="GET    /api/bootstrap/status" />
      <DocCurl cmd="POST   /api/bootstrap/admin" />
      <DocCurl cmd="POST   /api/bootstrap/validate-llm" />
      <DocCurl cmd="POST   /api/bootstrap/llm" />

      <h2>Auth</h2>
      <DocCurl cmd="-H 'authorization: Bearer $PROMPTSHEON_API_KEY'" />

      <DocNext href="/docs/cli" label="CLI" />
    </DocPage>
  );
}
