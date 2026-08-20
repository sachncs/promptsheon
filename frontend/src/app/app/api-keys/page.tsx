import { KeyRound } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ApiKeysStub() {
  return (
    <StubPage
      eyebrow="Admin"
      title="API keys"
      description="Programmatic access to the platform. Keys are scoped per role and can be revoked at any time."
      icon={KeyRound}
      primary={{
        title: 'No API keys',
        description: 'Generate a key to allow CI/CD or external services to interact with Promptsheon.',
        action: { label: 'Create key', href: '#' },
      }}
    />
  );
}
