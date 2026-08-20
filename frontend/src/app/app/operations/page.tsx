import { Activity } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function OperationsStub() {
  return (
    <StubPage
      eyebrow="Release"
      title="Operations hub"
      description="Runtime health of deployed capabilities. Latency, error rate, throughput, tool call patterns."
      icon={Activity}
      primary={{
        title: 'No deployments to monitor',
        description: 'Activate a capability release to start collecting runtime metrics here.',
        action: { label: 'Open releases', href: '/app/releases' },
      }}
    />
  );
}
