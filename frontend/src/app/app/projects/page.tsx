import { Boxes } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function ProjectsStub() {
  return (
    <StubPage
      eyebrow="Admin"
      title="Projects"
      description="Project folders sit inside workspaces. They group capabilities with shared eval suites and release policies."
      icon={Boxes}
      primary={{
        title: 'No projects to show',
        description: 'Open a workspace and create your first project.',
        action: { label: 'Open workspaces', href: '/app/workspaces' },
      }}
    />
  );
}
