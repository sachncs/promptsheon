import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { capabilityApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; projectId: string; }

export function CreateCapabilityModal({ open, onOpenChange, onSuccess, projectId }: Props) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const submit = async () => {
    await capabilityApi.create({ projectId, name, description });
    onSuccess(); onOpenChange(false);
    setName(''); setDescription('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Capability" disabled={!name} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="capability-name">Name</Label>
          <Input id="capability-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="capability-desc">Description</Label>
          <Textarea id="capability-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
