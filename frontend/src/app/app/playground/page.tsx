'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { ArrowLeft, Beaker, Play, Plus, Sparkles, Trash2 } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { playgroundApi, type PlaygroundRun } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup } from '@/components/brand/field';
import { ThemedSelect } from '@/components/brand/themed-select';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';

interface Variant {
  prompt: string;
  temperature: number;
}

const DEFAULT_PROMPT = 'You are a helpful assistant. Summarise the following customer feedback in 1 sentence.';
const DEFAULT_TEMP = 0.7;

export default function PlaygroundPage() {
  const session = useRequireSession();
  const [prompt, setPrompt] = useState(DEFAULT_PROMPT);
  const [model, setModel] = useState('gpt-4');
  const [provider, setProvider] = useState<'openai' | 'anthropic' | 'bedrock' | 'custom'>('custom');
  const [baseUrl, setBaseUrl] = useState('https://api.minimax.io/anthropic');
  const [apiKey, setApiKey] = useState('');
  const [temperature, setTemperature] = useState(DEFAULT_TEMP);
  const [variants, setVariants] = useState<Variant[]>([]);
  const [singleResult, setSingleResult] = useState<PlaygroundRun | null>(null);
  const [sweepResults, setSweepResults] = useState<
    Array<{ variant: Variant; status: 'fulfilled' | 'rejected'; value?: PlaygroundRun; error?: string }>
  >([]);

  const completeMutation = useMutation({
    mutationFn: () =>
      playgroundApi.complete({
        prompt,
        model,
        provider,
        temperature,
        ...(provider === 'custom' ? { baseUrl, apiKey } : {}),
      }),
    onSuccess: (r) => setSingleResult(r),
  });

  const sweepMutation = useMutation({
    mutationFn: () =>
      playgroundApi.sweep({
        base: {
          prompt,
          model,
          provider,
          ...(provider === 'custom' ? { baseUrl, apiKey } : {}),
        },
        variants,
      }),
    onSuccess: (r) => setSweepResults(r.variants),
  });

  const showSweep = useMemo(() => variants.length >= 1, [variants.length]);

  if (!session) return null;

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Control plane
        </Link>
      </div>

      <PageHeader
        eyebrow="Iteration"
        title="Playground"
        subtitle="Iterate a single prompt against the gateway with cache + fallback enabled, then run a parameter sweep to compare."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Prompt"
          description="The system prompt sent to the model. Cache key = prompt + model + temperature."
        />
        <div className="px-5 pb-5">
          <textarea
            rows={5}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            className="w-full rounded-md border border-border-subtle bg-surface-1 p-3 font-mono text-xs leading-relaxed"
          />
        </div>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Target" />
        <FieldGroup>
          <div className="grid grid-cols-1 gap-4 px-5 pb-5 md:grid-cols-4">
            <Field label="Provider" htmlFor="play-provider">
              <ThemedSelect
                value={provider}
                onValueChange={(v) => setProvider(v as typeof provider)}
                options={[
                  { value: 'openai', label: 'OpenAI' },
                  { value: 'anthropic', label: 'Anthropic' },
                  { value: 'bedrock', label: 'Bedrock' },
                  { value: 'custom', label: 'Custom endpoint' },
                ]}
              />
            </Field>
            <Field label="Model" htmlFor="play-model">
              <input
                id="play-model"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-sm"
              />
            </Field>
            <Field
              label="Temperature"
              htmlFor="play-temp"
              hint="0 = deterministic, 2 = chaotic"
            >
              <input
                id="play-temp"
                type="number"
                step={0.1}
                min={0}
                max={2}
                value={temperature}
                onChange={(e) => setTemperature(Number(e.target.value))}
                className="w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-sm font-mono"
              />
            </Field>
            <Field label="" htmlFor="play-run">
              <Button
                id="play-run"
                onClick={() => completeMutation.mutate()}
                disabled={completeMutation.isPending || !prompt}
              >
                <Play className="mr-1.5 h-3.5 w-3.5" />
                {completeMutation.isPending ? 'Running…' : 'Run'}
              </Button>
            </Field>
            {provider === 'custom' && (
              <>
                <Field label="Base URL" htmlFor="play-baseurl">
                  <input
                    id="play-baseurl"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    className="w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-sm font-mono"
                  />
                </Field>
                <Field label="API key" htmlFor="play-apikey" hint="not stored">
                  <input
                    id="play-apikey"
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    className="w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-sm font-mono"
                    autoComplete="off"
                  />
                </Field>
              </>
            )}
          </div>
        </FieldGroup>
      </Surface>

      {singleResult && (
        <RunCard run={singleResult} variant={{ prompt, temperature }} />
      )}
      {completeMutation.error && (
        <Surface>
          <div className="px-5 py-4 text-sm text-destructive">
            {String((completeMutation.error as Error).message)}
          </div>
        </Surface>
      )}

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Parameter sweep"
          description="Run the prompt across N variants in parallel — different temperatures, or different prompts — and compare latency, tokens, cost, and output side-by-side."
          actions={
            <Button size="sm" variant="ghost" onClick={() => setVariants((v) => [...v, { prompt, temperature }])}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              Capture current as variant
            </Button>
          }
        />
        <ul className="divide-y divide-border-subtle">
          {variants.length === 0 && (
            <li className="px-5 py-6">
              <EmptyState
                className="border-0 bg-transparent shadow-none p-0"
                icon={Beaker}
                title="No variants yet"
                description="Click 'Capture current as variant' to add the current prompt + temperature to the sweep, then run."
              />
            </li>
          )}
          {variants.map((v, idx) => (
            <li key={idx} className="flex items-start gap-3 px-5 py-3 text-sm">
              <span className="mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-surface-2 font-mono text-[10px] text-text-muted">
                {idx + 1}
              </span>
              <div className="flex-1 min-w-0">
                <div className="flex items-baseline gap-2">
                  <span className="font-mono text-xs text-text-muted">temp={v.temperature}</span>
                  <span className="text-text-muted">·</span>
                  <span className="truncate text-text-default">{v.prompt.slice(0, 80)}{v.prompt.length > 80 ? '…' : ''}</span>
                </div>
              </div>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setVariants((vs) => vs.filter((_, i) => i !== idx))}
                aria-label={`Remove variant ${idx + 1}`}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </li>
          ))}
        </ul>
        {showSweep && (
          <div className="border-t border-border-subtle px-5 py-3">
            <Button
              onClick={() => sweepMutation.mutate()}
              disabled={sweepMutation.isPending}
            >
              <Sparkles className="mr-1.5 h-3.5 w-3.5" />
              {sweepMutation.isPending ? 'Running sweep…' : `Run ${variants.length} variant${variants.length === 1 ? '' : 's'}`}
            </Button>
          </div>
        )}
      </Surface>

      {sweepResults.length > 0 && (
        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title="Sweep results"
            description="Top-to-bottom matches variant order. Hit the cache on subsequent identical runs."
          />
          <ul className="divide-y divide-border-subtle">
            {sweepResults.map((r, idx) => (
              <li key={idx} className="px-5 py-4">
                <div className="mb-2 flex flex-wrap items-baseline gap-2 text-xs text-text-muted">
                  <span className="font-mono text-text-subtle">#{idx + 1}</span>
                  <span>temp={r.variant.temperature}</span>
                  <span>·</span>
                  <span>{r.variant.prompt.slice(0, 64)}{r.variant.prompt.length > 64 ? '…' : ''}</span>
                <StatusPill
                  kind={r.status === 'fulfilled' ? 'active' : 'error'}
                  label={r.status === 'fulfilled' ? 'ok' : 'error'}
                />
                </div>
                {r.value ? (
                  <ResultBlock run={r.value} />
                ) : (
                  <div className="rounded-md bg-surface-0 p-3 text-xs text-destructive">
                    {r.error}
                  </div>
                )}
              </li>
            ))}
          </ul>
        </Surface>
      )}
    </div>
  );
}

function RunCard({ run, variant }: { run: PlaygroundRun; variant: Variant }) {
  return (
    <Surface padded={false}>
      <SurfaceHeader
        className="px-5 pt-5"
        title="Single run"
        description={`temp=${variant.temperature} · ${run.provider}/${run.model}`}
      />
      <ResultBlock run={run} />
    </Surface>
  );
}

function ResultBlock({ run }: { run: PlaygroundRun }) {
  return (
    <div className="px-5 pb-5">
      <pre className="max-h-80 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
        {run.content}
      </pre>
      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-text-muted">
        <span>{run.promptTokens.toLocaleString()} prompt tokens</span>
        <span>·</span>
        <span>{run.completionTokens.toLocaleString()} completion tokens</span>
        <span>·</span>
        <span>${run.costUsd.toFixed(4)}</span>
        <span>·</span>
        <span>{run.latencyMs} ms</span>
        {run.cacheHit && (
          <span className="rounded-full bg-brand/10 px-2 py-0.5 text-[10px] uppercase tracking-wider text-brand-highlight">
            cache hit
          </span>
        )}
      </div>
    </div>
  );
}
