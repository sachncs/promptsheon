import { useState, type ReactNode } from 'react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  submitLabel?: string;
  disabled?: boolean;
  onSubmit: () => Promise<void> | void;
  children: ReactNode;
}

export function FormDialog({ open, onOpenChange, title, submitLabel = 'Create', disabled, onSubmit, children }: Props) {
  const [pending, setPending] = useState(false);
  const handleSubmit = async () => {
    setPending(true);
    try { await onSubmit(); } finally { setPending(false); }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader><DialogTitle>{title}</DialogTitle></DialogHeader>
        {children}
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={disabled || pending}>{submitLabel}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
