import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { projectApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; workspaceId: string; }

export function CreateProjectModal({ open, onOpenChange, onSuccess, workspaceId }: Props) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const submit = async () => {
    await projectApi.create({ workspaceId, name, description });
    onSuccess(); onOpenChange(false);
    setName(''); setDescription('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Project" disabled={!name} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="project-name">Name</Label>
          <Input id="project-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="project-desc">Description</Label>
          <Textarea id="project-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
