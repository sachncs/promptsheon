import { ShieldCheck } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ApprovalsStub() {
  return (
    <StubPage
      eyebrow="Quality"
      title="Approvals"
      description="Maker-checker approval queue. Releases need a second reviewer before going to canary."
      icon={ShieldCheck}
      primary={{
        title: 'No approvals queued',
        description: 'When a release enters review, it appears here for a second pair of eyes.',
        action: { label: 'Browse releases', href: '/app/releases' },
      }}
    />
  );
}
