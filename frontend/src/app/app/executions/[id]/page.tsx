import { Play } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ExecutionsStub() {
  return (
    <StubPage
      eyebrow="Capabilities"
      title="Executions"
      description="Live executions against activated releases. Each run is traced, sampled, and logged."
      icon={Play}
      primary={{
        title: 'No executions to show',
        description: 'Trigger a test run against an active release to populate this view.',
        action: { label: 'Open releases', href: '/app/releases' },
      }}
    />
  );
}
