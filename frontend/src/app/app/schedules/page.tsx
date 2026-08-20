import { CalendarClock } from 'lucide-react';
import { StubPage } from '@/components/brand/stub-page';

export default function SchedulesStub() {
  return (
    <StubPage
      eyebrow="Release"
      title="Schedules"
      description="Cron-based schedules for eval runs, release rotations, and policy enforcement windows."
      icon={CalendarClock}
      primary={{
        title: 'No schedules running',
        description: 'Schedule a nightly eval run or a release rotation.',
        action: { label: 'Create schedule', href: '#' },
      }}
    />
  );
}
