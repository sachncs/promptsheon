import { ShieldCheck } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ReleaseApprovalStub() {
  return (
    <StubPage
      eyebrow="Quality"
      title="Release approval"
      description="Approve or reject a release by inspecting its diff, eval results, and lineage."
      icon={ShieldCheck}
      primary={{
        title: 'Approval detail',
        description: 'Open the release to see its diff and eval outcomes before signing off.',
        action: { label: 'Browse releases', href: '/app/releases' },
      }}
    />
  );
}
