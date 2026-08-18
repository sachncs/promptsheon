import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { releaseApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; capabilityId: string; }

export function CreateReleaseModal({ open, onOpenChange, onSuccess, capabilityId }: Props) {
  const [capabilityVersion, setCapabilityVersion] = useState('1');
  const [manifest, setManifest] = useState('');
  const [environment, setEnvironment] = useState('dev');

  const submit = async () => {
    await releaseApi.create({
      capabilityId,
      capabilityVersion: Number(capabilityVersion),
      capabilityVersionId: null,
      manifest,
      environment,
    });
    onSuccess(); onOpenChange(false);
    setManifest(''); setEnvironment('dev');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Release" disabled={!manifest} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="release-version">Capability Version</Label>
          <Input id="release-version" type="number" min="1" value={capabilityVersion} onChange={(e) => setCapabilityVersion(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="release-manifest">Manifest</Label>
          <Textarea id="release-manifest" rows={6} value={manifest} onChange={(e) => setManifest(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="release-env">Environment</Label>
          <Input id="release-env" value={environment} onChange={(e) => setEnvironment(e.target.value)} placeholder="dev | staging | prod" />
        </div>
      </div>
    </FormDialog>
  );
}
