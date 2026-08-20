import { Webhook } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function WebhooksStub() {
  return (
    <StubPage
      eyebrow="Admin"
      title="Webhooks"
      description="Outbound webhooks for capability lifecycle events. Each webhook is signed with HMAC and stores its delivery history."
      icon={Webhook}
      primary={{
        title: 'No webhooks configured',
        description: 'Add a webhook to receive capability lifecycle events with HMAC verification.',
        action: { label: 'Add webhook', href: '#' },
      }}
    />
  );
}
