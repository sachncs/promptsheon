'use client';

import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { ArrowRight, Compass, RotateCcw, Sparkles } from 'lucide-react';
import { compilerApi } from '@/lib/api';
import type { Manifest } from '@promptsheon/shared';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { EmptyState } from '@/components/brand/empty-state';
import { getErrorMessage } from '@/lib/errors';

type Mode = 'compile' | 'decompile';

interface CompileResult {
  manifest?: Manifest;
  prompt?: string;
}

export default function CompilerPage() {
  const session = useRequireSession();
  const [mode, setMode] = useState<Mode>('compile');
  const [input, setInput] = useState('');
  const [result, setResult] = useState<CompileResult | null>(null);

  const mutation = useMutation({
    mutationFn: async () => {
      let manifest: unknown;
      try {
        manifest = JSON.parse(input) as unknown;
      } catch {
        throw new Error('Input must be valid JSON.');
      }
      if (mode === 'compile') {
        const r = await compilerApi.compile(manifest);
        return { manifest: r.data } satisfies CompileResult;
      }
      const r = await compilerApi.decompile(manifest);
      return { prompt: r.data } satisfies CompileResult;
    },
    onSuccess: (data) => setResult(data),
  });

  if (!session) return null;

  const placeholder =
    mode === 'compile'
    ? 'Paste a valid manifest JSON object to compile it with the reasoning compiler.'
      : 'Paste a compiled manifest JSON object to round-trip it back into a natural-language description.';

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capabilities"
        title="Compiler"
        subtitle="The compiler is the source of reproducible artifacts. Prompts compile into deterministic capability manifests; manifests decompile back into a prompt for review."
        actions={
          <div className="inline-flex rounded-lg border border-border-subtle bg-surface-1 p-0.5">
            <button
              type="button"
              onClick={() => { setMode('compile'); setResult(null); }}
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                mode === 'compile' ? 'bg-brand text-brand-foreground' : 'text-text-muted hover:text-text-default'
              }`}
            >
              <Sparkles className="size-3.5" /> Compile
            </button>
            <button
              type="button"
              onClick={() => { setMode('decompile'); setResult(null); }}
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                mode === 'decompile' ? 'bg-brand text-brand-foreground' : 'text-text-muted hover:text-text-default'
              }`}
            >
              <RotateCcw className="size-3.5" /> Decompile
            </button>
          </div>
        }
      />

      <div className="grid gap-5 lg:grid-cols-2">
        <Surface>
          <SurfaceHeader
            title={mode === 'compile' ? 'Prompt' : 'Manifest'}
            description={
              mode === 'compile'
                ? 'Natural-language capability description.'
                : 'Compiled capability manifest (YAML or JSON).'
            }
          />
          <Textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={placeholder}
            className="min-h-72 font-mono text-sm"
          />
          <div className="mt-4 flex items-center justify-between">
            <div className="text-xs text-text-subtle">
              {input.length.toLocaleString()} characters
            </div>
            <Button
              onClick={() => mutation.mutate()}
              disabled={!input.trim() || mutation.isPending}
            >
              {mutation.isPending ? 'Running…' : (
                <>
                  {mode === 'compile' ? 'Compile' : 'Decompile'}
                  <ArrowRight className="ml-1.5 size-4" />
                </>
              )}
            </Button>
          </div>
          {mutation.isError && (
            <div role="alert" className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {getErrorMessage(mutation.error, 'The compiler request failed. Try again.')}
            </div>
          )}
        </Surface>

        <Surface>
          <SurfaceHeader
            title={mode === 'compile' ? 'Manifest' : 'Prompt'}
            description={
              mode === 'compile'
                ? 'The compiled capability manifest.'
                : 'A natural-language reconstruction of the manifest.'
            }
          />
          {!result ? (
            <EmptyState
              icon={Compass}
              title="No output yet"
              description={`${mode === 'compile' ? 'Compile' : 'Decompile'} a ${mode === 'compile' ? 'prompt' : 'manifest'} to see the result here.`}
              className="border-0 bg-transparent shadow-none p-12"
            />
          ) : (
            <div className="space-y-3">
              <pre className="max-h-96 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                {mode === 'compile' ? JSON.stringify(result.manifest, null, 2) : (result.prompt ?? '')}
              </pre>
            </div>
          )}
        </Surface>
      </div>

      <Surface>
        <div className="text-xs text-text-subtle">
          The compiler is deterministic: same prompt in, same manifest hash out. Save a manifest from the
          editor to round-trip through the compiler; the CAS stores every artifact by content hash.
        </div>
      </Surface>
    </div>
  );
}
