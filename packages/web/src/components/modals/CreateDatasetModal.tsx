import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { datasetApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; capabilityId: string; }

export function CreateDatasetModal({ open, onOpenChange, onSuccess, capabilityId }: Props) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const submit = async () => {
    await datasetApi.create({ capabilityId, name, description });
    onSuccess(); onOpenChange(false);
    setName(''); setDescription('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Dataset" disabled={!name} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="dataset-name">Name</Label>
          <Input id="dataset-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="dataset-desc">Description</Label>
          <Textarea id="dataset-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
