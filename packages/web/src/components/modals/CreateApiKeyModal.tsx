import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { apiKeyApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; }

export function CreateApiKeyModal({ open, onOpenChange, onSuccess }: Props) {
  const [name, setName] = useState('');
  const [role, setRole] = useState<'admin' | 'editor' | 'reader'>('reader');

  const submit = async () => {
    await apiKeyApi.create({ name, role });
    onSuccess(); onOpenChange(false);
    setName(''); setRole('reader');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create API Key" disabled={!name} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="apikey-name">Name</Label>
          <Input id="apikey-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Role</Label>
          <Select value={role} onValueChange={(v) => setRole(v as typeof role)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="admin">Admin</SelectItem>
              <SelectItem value="editor">Editor</SelectItem>
              <SelectItem value="reader">Reader</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </FormDialog>
  );
}
