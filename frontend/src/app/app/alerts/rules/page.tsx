import { Bell } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function AlertRulesStub() {
  return (
    <StubPage
      eyebrow="Release"
      title="Alert rules"
      description="Define when alerts fire: eval regression thresholds, latency, error rate, approval-window breaches."
      icon={Bell}
      primary={{
        title: 'No alert rules yet',
        description: 'Define a rule — for example, "fail release activation when eval score drops below 0.85 for two consecutive runs".',
        action: { label: 'Create rule', href: '#' },
      }}
    />
  );
}
