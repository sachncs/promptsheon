import { Flag } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function FeatureFlagsStub() {
  return (
    <StubPage
      eyebrow="Capabilities"
      title="Feature flags"
      description="Capability-level feature flags. Toggle which DAG branches run for which users; promote features safely."
      icon={Flag}
      primary={{
        title: 'No flags configured',
        description: 'Create a flag to gate risky capability paths behind an environment rollout.',
        action: { label: 'Create flag', href: '#' },
      }}
    />
  );
}
