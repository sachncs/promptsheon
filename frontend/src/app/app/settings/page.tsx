import { Cog } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function SettingsStub() {
  return (
    <StubPage
      eyebrow="Admin"
      title="Settings"
      description="Platform-wide configuration. Defaults for LLM, audit retention, canary thresholds, and observability."
      icon={Cog}
      primary={{
        title: 'No platform settings configured',
        description: 'Defaults are loaded from environment variables. Override them here at runtime.',
        action: { label: 'Configure settings', href: '#' },
      }}
    />
  );
}
