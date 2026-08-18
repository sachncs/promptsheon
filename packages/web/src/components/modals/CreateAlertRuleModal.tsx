import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { alertApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; }

const SEVERITIES: Array<'info' | 'warning' | 'critical'> = ['info', 'warning', 'critical'];

export function CreateAlertRuleModal({ open, onOpenChange, onSuccess }: Props) {
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [severity, setSeverity] = useState<'info' | 'warning' | 'critical'>('warning');
  const [threshold, setThreshold] = useState('0');

  const submit = async () => {
    await alertApi.createRule({ name, type, severity, threshold: Number(threshold) });
    onSuccess(); onOpenChange(false);
    setName(''); setType(''); setSeverity('warning'); setThreshold('0');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Create Alert Rule" disabled={!name || !type} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2"><Label htmlFor="alert-name">Name</Label>
          <Input id="alert-name" value={name} onChange={(e) => setName(e.target.value)} /></div>
        <div className="space-y-2"><Label htmlFor="alert-type">Type</Label>
          <Input id="alert-type" value={type} onChange={(e) => setType(e.target.value)} /></div>
        <div className="space-y-2"><Label>Severity</Label>
          <Select value={severity} onValueChange={(v) => setSeverity(v as typeof severity)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>{SEVERITIES.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}</SelectContent>
          </Select></div>
        <div className="space-y-2"><Label htmlFor="alert-threshold">Threshold</Label>
          <Input id="alert-threshold" type="number" value={threshold} onChange={(e) => setThreshold(e.target.value)} /></div>
      </div>
    </FormDialog>
  );
}
