import { Users } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function UsersStub() {
  return (
    <StubPage
      eyebrow="Admin"
      title="Users"
      description="Manage the people and service accounts that interact with the platform. Roles: admin, approver, editor, viewer."
      icon={Users}
      primary={{
        title: 'No users yet',
        description: 'Invite teammates and assign their roles in the organisation.',
        action: { label: 'Invite user', href: '#' },
      }}
    />
  );
}
