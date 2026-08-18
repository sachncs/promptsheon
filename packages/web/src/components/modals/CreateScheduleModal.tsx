import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { scheduleApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; workspaceId: string; releaseId: string; }

export function CreateScheduleModal({ open, onOpenChange, onSuccess, workspaceId, releaseId }: Props) {
  const [kind, setKind] = useState('');
  const [cron, setCron] = useState('');

  const submit = async () => {
    await scheduleApi.create({ workspaceId, releaseId, kind, cron });
    onSuccess(); onOpenChange(false);
    setKind(''); setCron('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Schedule" disabled={!kind || !cron} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="schedule-kind">Kind</Label>
          <Input id="schedule-kind" value={kind} onChange={(e) => setKind(e.target.value)} placeholder="invoke | eval | ..." />
        </div>
        <div className="space-y-2">
          <Label htmlFor="schedule-cron">Cron Expression</Label>
          <Input id="schedule-cron" value={cron} onChange={(e) => setCron(e.target.value)} placeholder="0 * * * *" />
        </div>
      </div>
    </FormDialog>
  );
}
