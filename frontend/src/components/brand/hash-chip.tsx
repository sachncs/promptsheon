'use client';

import { Check, Copy } from 'lucide-react';
import * as React from 'react';
import { cn } from '@/lib/utils';

export function HashChip({
  hash,
  length = 10,
  className,
  mono = true,
}: {
  hash: string;
  length?: number;
  className?: string | undefined;
  mono?: boolean;
}) {
  const [copied, setCopied] = React.useState(false);
  const truncated = hash.length > length ? `${hash.slice(0, length)}…` : hash;

  function copy(): void {
    if (typeof navigator === 'undefined' || !navigator.clipboard) return;
    void navigator.clipboard.writeText(hash).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    });
  }

  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border border-border-subtle bg-surface-1 px-2 py-0.5 text-xs text-text-muted transition-colors hover:border-border-strong hover:text-text-default',
        mono && 'font-mono tracking-tight',
        className,
      )}
      aria-label="Copy hash"
    >
      <span>{truncated}</span>
      {copied ? <Check className="h-3 w-3 text-success" /> : <Copy className="h-3 w-3" />}
    </button>
  );
}
