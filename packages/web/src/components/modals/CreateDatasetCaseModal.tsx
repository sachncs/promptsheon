import { useState } from 'react';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { datasetApi } from '@/lib/api';
import { FormDialog } from './FormDialog';

interface Props { open: boolean; onOpenChange: (open: boolean) => void; onSuccess: () => void; datasetId: string; }

export function CreateDatasetCaseModal({ open, onOpenChange, onSuccess, datasetId }: Props) {
  const [inputs, setInputs] = useState('');
  const [expected, setExpected] = useState('');

  const submit = async () => {
    await datasetApi.addCase(datasetId, { inputs, expected });
    onSuccess(); onOpenChange(false);
    setInputs(''); setExpected('');
  };

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="Add Dataset Case" submitLabel="Add" disabled={!inputs || !expected} onSubmit={submit}>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="case-inputs">Inputs (JSON)</Label>
          <Textarea id="case-inputs" rows={4} value={inputs} onChange={(e) => setInputs(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="case-expected">Expected (JSON)</Label>
          <Textarea id="case-expected" rows={4} value={expected} onChange={(e) => setExpected(e.target.value)} />
        </div>
      </div>
    </FormDialog>
  );
}
