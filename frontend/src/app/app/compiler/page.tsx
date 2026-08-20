import { Compass } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function CompilerStub() {
  return (
    <StubPage
      eyebrow="Capabilities"
      title="Compiler"
      description="Natural-language prompts compile into deterministic capability manifests. The compiler is the source of reproducible artifacts."
      icon={Compass}
      primary={{
        title: 'Open the compiler',
        description: 'Author your first capability by describing what it should do.',
        action: { label: 'Compile a prompt', href: '#' },
      }}
    />
  );
}
