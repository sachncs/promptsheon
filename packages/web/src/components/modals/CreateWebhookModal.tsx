import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { webhookApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; }

export function CreateWebhookModal({ open, onOpenChange, onSuccess }: Props) {
  const [url, setUrl] = useState('');
  const [events, setEvents] = useState('');

  const submit = async () => {
    await webhookApi.create({ url, events: events.split(',').map((e) => e.trim()).filter(Boolean) });
    onSuccess(); onOpenChange(false);
    setUrl(''); setEvents('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Webhook" disabled={!url || !events} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="webhook-url">URL</Label>
          <Input id="webhook-url" type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/hook" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="webhook-events">Events (comma-separated)</Label>
          <Input id="webhook-events" value={events} onChange={(e) => setEvents(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
