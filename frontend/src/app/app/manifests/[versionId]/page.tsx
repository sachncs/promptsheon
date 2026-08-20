import { ScrollText } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ManifestStub() {
  return (
    <StubPage
      eyebrow="Capabilities"
      title="Manifest"
      description="A content-addressed, compiled artifact. Its hash is its identity; its lineage is preserved."
      icon={ScrollText}
      primary={{
        title: 'No manifest selected',
        description: 'Open a capability to view its compiled manifests and hashes.',
        action: { label: 'Open registry', href: '/app/capabilities' },
      }}
    />
  );
}
