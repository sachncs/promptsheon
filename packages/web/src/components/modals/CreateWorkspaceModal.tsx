import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { workspaceApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; }

export function CreateWorkspaceModal({ open, onOpenChange, onSuccess }: Props) {
  const [name, setName] = useState('');
  const [organization, setOrganization] = useState('');

  const submit = async () => {
    await workspaceApi.create({ name, organization });
    onSuccess(); onOpenChange(false);
    setName(''); setOrganization('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Workspace" disabled={!name} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="workspace-name">Name</Label>
          <Input id="workspace-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="workspace-org">Organization (optional)</Label>
          <Input id="workspace-org" value={organization} onChange={(e) => setOrganization(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
