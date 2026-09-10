'use client';

import { useRouter } from 'next/navigation';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight, Bot, CheckCircle2, KeyRound, Loader2, PlugZap, ShieldAlert, User } from 'lucide-react';
import * as React from 'react';
import { LogoMark } from '@/brand/logo-mark';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { StepIndicator } from '@/components/brand/step-indicator';
import { bootstrapApi, toSession } from '@/lib/bootstrap';
import { getSession, setSession } from '@/lib/session';
import { cn } from '@/lib/utils';

const steps = [
  { id: 'welcome', label: 'Welcome', icon: Bot },
  { id: 'admin', label: 'Identity', icon: User },
  { id: 'llm', label: 'Provider', icon: KeyRound },
  { id: 'finish', label: 'Finish', icon: CheckCircle2 },
];

type Provider = 'openai' | 'anthropic' | 'bedrock' | 'custom';

const providerDefaults: Record<Provider, { model: string; placeholder: string; baseUrl: string; apiStyle: 'anthropic' | 'openai' }> = {
  openai:    { model: 'gpt-4o-mini',                       placeholder: 'sk-…',                                                                              baseUrl: 'https://api.openai.com',                    apiStyle: 'openai' },
  anthropic: { model: 'claude-3-5-haiku-latest',          placeholder: 'sk-ant-…',                                                                           baseUrl: 'https://api.anthropic.com',                 apiStyle: 'anthropic' },
  bedrock:   { model: 'anthropic.claude-3-5-sonnet-20241022-v2:0', placeholder: '',                                                                       baseUrl: '',                                          apiStyle: 'anthropic' },
  custom:    { model: '',                                  placeholder: 'paste your model id',                                                                baseUrl: 'https://api.minimax.io/anthropic',          apiStyle: 'anthropic' },
};

const providerLabels: Record<Provider, { title: string; hint: string }> = {
  openai:    { title: 'OpenAI',          hint: 'gpt-4o, gpt-4o-mini, gpt-4.1' },
  anthropic: { title: 'Anthropic',       hint: 'claude-3-5-sonnet, claude-3-5-haiku' },
  bedrock:   { title: 'AWS Bedrock',     hint: 'Claude on AWS' },
  custom:    { title: 'Custom endpoint', hint: 'Any OpenAI- or Anthropic-compatible URL' },
};

export default function OnboardingPage() {
  const router = useRouter();
  const status = useQuery({ queryKey: ['bootstrap', 'status'], queryFn: () => bootstrapApi.status() });
  const [index, setIndex] = React.useState(0);

  React.useEffect(() => {
    if (!status.data) return;
    // Bootstrap is complete. If we already have a session, head straight
    // to the app. If localStorage was cleared (or never written — the
    // admin step sets the session but localStorage can be wiped between
    // dev runs), re-establish the session by fetching the admin record
    // and then redirect.
    if (!status.data.needsAdmin && !status.data.needsLlm) {
      const existing = getSession();
      if (existing) {
        router.replace('/app');
        return;
      }
      bootstrapApi.admin()
        .then((data) => {
          setSession(toSession(data, status.data?.provider ?? null));
          router.replace('/app');
        })
        .catch(() => {
          // No admin or no org — leave the user on the onboarding UI so
          // they can run the steps. (If we got here, status must be stale.)
        });
    }
  }, [status.data, router]);

  if (status.isLoading) {
    return (
      <div className="flex h-64 items-center justify-center text-text-muted text-sm">
        <Loader2 className="h-4 w-4 animate-spin mr-2" />
        Loading environment state…
      </div>
    );
  }

  return (
    <div className="relative">
      <div className="ps-aurora-bg pointer-events-none absolute -inset-x-12 -top-12 -bottom-12 -z-10 rounded-[3rem]" aria-hidden />
      <div className="rounded-2xl border border-border-subtle bg-surface-1 p-8 shadow-2 sm:p-10">
        <StepIndicator steps={steps} currentIndex={index} />
        <div className="mt-8">
          {index === 0 && <Welcome onNext={() => setIndex(1)} />}
          {index === 1 && (
            <AdminStep
              onBack={() => setIndex(0)}
              onNext={() => setIndex(2)}
            />
          )}
          {index === 2 && (
            <LlmStep
              presetProvider={status.data?.provider ?? undefined}
              onBack={() => setIndex(1)}
              onNext={() => setIndex(3)}
            />
          )}
          {index === 3 && <Finish onDone={() => router.push('/app')} />}
        </div>
      </div>
    </div>
  );
}

function Welcome({ onNext }: { onNext: () => void }) {
  return (
    <section className="text-center">
      <LogoMark size={56} className="mx-auto" />
      <h1 className="mt-5 text-3xl font-semibold tracking-tight text-text-strong">
        Set up Promptsheon.
      </h1>
      <p className="mt-3 text-text-muted max-w-md mx-auto">
        Configure your organisation, connect a model provider, and arrive at the
        control plane with a governed workspace. About sixty seconds.
      </p>
      <ul className="mt-7 mx-auto max-w-md space-y-2 text-left text-sm text-text-muted">
        {[
          'Create the first admin and organisation.',
          'Connect any OpenAI- or Anthropic-compatible provider, or AWS Bedrock.',
          'Land in a workspace ready to author capabilities.',
        ].map((line) => (
          <li key={line} className="flex items-start gap-2">
            <CheckCircle2 className="mt-0.5 h-4 w-4 text-success" />
            <span>{line}</span>
          </li>
        ))}
      </ul>
      <Button size="lg" onClick={onNext} className="mt-8">
        Begin setup
        <ArrowRight className="ml-1.5 h-4 w-4" />
      </Button>
    </section>
  );
}

function AdminStep({ onBack, onNext }: { onBack: () => void; onNext: () => void }) {
  const [adminName, setAdminName] = React.useState('');
  const [adminEmail, setAdminEmail] = React.useState('');
  const [orgName, setOrgName] = React.useState('');
  const [orgSlug, setOrgSlug] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);

  const submit = useMutation({
    mutationFn: () => bootstrapApi.createAdmin({
      adminName: adminName.trim(),
      adminEmail: adminEmail.trim(),
      orgName: orgName.trim(),
      orgSlug: orgSlug.trim() || undefined,
    }),
    onSuccess: (data) => {
      setError(null);
      onNext();
      // Store a partial session; finalised at finish step.
      setSession(toSession(data, null));
    },
    onError: (err: Error) => setError(err.message),
  });

  return (
    <section>
      <Header title="Your workspace" subtitle="Create the first admin and an organisation. Both names are editable later." />
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Admin name" htmlFor="admin-name">
          <Input id="admin-name" autoFocus value={adminName} onChange={(e) => setAdminName(e.target.value)} placeholder="Ada Lovelace" />
        </Field>
        <Field label="Admin email" htmlFor="admin-email">
          <Input id="admin-email" type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} placeholder="ada@example.com" />
        </Field>
        <Field label="Organisation name" htmlFor="org-name">
          <Input id="org-name" value={orgName} onChange={(e) => setOrgName(e.target.value)} placeholder="Acme AI" />
        </Field>
        <Field
          label="Organisation slug"
          htmlFor="org-slug"
          hint="Used in URLs. Leave blank to auto-generate."
        >
          <Input id="org-slug" value={orgSlug} onChange={(e) => setOrgSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))} placeholder="acme-ai" />
        </Field>
      </div>
      {error && <Banner tone="error" message={error} />}
      <Footer
        back={{ onClick: onBack, label: 'Back' }}
        next={{
          disabled: !adminName.trim() || !adminEmail.includes('@') || !orgName.trim(),
          loading: submit.isPending,
          label: 'Continue',
          onClick: () => submit.mutate(),
        }}
      />
    </section>
  );
}

function LlmStep({
  presetProvider,
  onBack,
  onNext,
}: {
  presetProvider?: string | undefined;
  onBack: () => void;
  onNext: () => void;
}) {
  const [provider, setProvider] = React.useState<Provider>(
    (presetProvider as Provider) ?? 'openai',
  );
  const [model, setModel] = React.useState(providerDefaults.openai.model);
  const [apiKey, setApiKey] = React.useState('');
  const [bedrockRegion, setBedrockRegion] = React.useState('us-east-1');
  const [bedrockAccess, setBedrockAccess] = React.useState('');
  const [bedrockSecret, setBedrockSecret] = React.useState('');
  const [baseUrl, setBaseUrl] = React.useState(providerDefaults.openai.baseUrl);
  const [formError, setFormError] = React.useState<string | null>(null);

  const [probeState, setProbeState] = React.useState<
    | { kind: 'idle' }
    | { kind: 'probing' }
    | { kind: 'ok'; latencyMs: number; model: string }
    | { kind: 'error'; message: string }
  >({ kind: 'idle' });

  React.useEffect(() => {
    setModel(providerDefaults[provider].model);
    setBaseUrl(providerDefaults[provider].baseUrl);
    setProbeState({ kind: 'idle' });
    setFormError(null);
  }, [provider]);

  async function probe(): Promise<void> {
    setFormError(null);
    if (!model.trim()) {
      setFormError('Please add a model name before testing the connection.');
      return;
    }
    if (provider === 'bedrock') {
      if (!bedrockAccess.trim() || !bedrockSecret.trim()) {
        setFormError('Enter the AWS access key id and secret before testing.');
        return;
      }
    } else {
      if (!apiKey.trim()) {
        setFormError('Enter the API key before testing the connection.');
        return;
      }
      if (provider === 'custom' && !baseUrl.trim()) {
        setFormError('Enter the base URL for the custom provider.');
        return;
      }
    }
    setProbeState({ kind: 'probing' });
    try {
      const baseReq: Parameters<typeof bootstrapApi.validateLlm>[0] = { provider, model: model.trim() };
      if (provider === 'bedrock') {
        baseReq.bedrock = { region: bedrockRegion, accessKeyId: bedrockAccess.trim(), secretAccessKey: bedrockSecret.trim() };
      } else {
        baseReq.apiKey = apiKey.trim();
        if (provider === 'custom' && baseUrl.trim()) baseReq.baseUrl = baseUrl.trim();
      }
      const res = await bootstrapApi.validateLlm(baseReq);
      setProbeState({ kind: 'ok', latencyMs: res.latencyMs, model: res.model });
    } catch (err) {
      setProbeState({ kind: 'error', message: err instanceof Error ? err.message : String(err) });
    }
  }

  const save = useMutation({
    mutationFn: async () => {
      if (!model.trim()) throw new Error('Please add a model name before saving.');
      const req: Parameters<typeof bootstrapApi.saveLlm>[0] = { provider, model: model.trim() };
      if (provider === 'bedrock') {
        req.bedrock = { region: bedrockRegion, accessKeyId: bedrockAccess.trim(), secretAccessKey: bedrockSecret.trim() };
      } else {
        req.apiKey = apiKey.trim();
        if (provider === 'custom' && baseUrl.trim()) req.baseUrl = baseUrl.trim();
      }
      await bootstrapApi.saveLlm(req);
      const existing = JSON.parse(window.localStorage.getItem('promptsheon:session:v1') ?? 'null');
      if (existing) {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({ ...existing, provider }),
        );
      }
    },
    onSuccess: () => onNext(),
    onError: (err: Error) => setProbeState({ kind: 'error', message: err.message }),
  });

  const disabled = probeState.kind !== 'ok' || save.isPending;

  return (
    <section>
      <Header title="Connect a model provider" subtitle="Promptsheon delegates every agent call through the provider you choose. The key is stored encrypted at rest and only ever returned as a masked value." />

      <div className="grid gap-2 sm:grid-cols-4">
        {(Object.keys(providerDefaults) as Provider[]).map((p) => (
          <button
            key={p}
            type="button"
            onClick={() => setProvider(p)}
            className={cn(
              'rounded-xl border px-3 py-3 text-left transition-colors',
              provider === p
                ? 'border-brand bg-brand/10 text-text-strong'
                : 'border-border-subtle bg-surface-2 text-text-muted hover:border-border-strong hover:text-text-default',
            )}
          >
            <div className="text-sm font-semibold">{providerLabels[p].title}</div>
            <div className="mt-0.5 text-xs text-text-muted">
              {providerLabels[p].hint}
            </div>
          </button>
        ))}
      </div>

      <div className="mt-5 grid gap-4">
        {provider === 'custom' && (
          <Field label="Base URL" htmlFor="base-url" hint="Endpoint of any OpenAI- or Anthropic-compatible API (for example: https://api.minimax.io/anthropic).">
            <Input
              id="base-url"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="https://api.minimax.io/anthropic"
              mono
            />
          </Field>
        )}
        <Field
          label="Model name"
          htmlFor="model-id"
          hint={provider === 'custom' ? 'The exact model id your endpoint expects.' : 'Pick any model your provider offers; this is just a sensible default.'}
        >
          <Input
            id="model-id"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder={providerDefaults[provider].model || 'e.g. MiniMax-M2.7-highspeed'}
            mono
          />
        </Field>
        {provider === 'bedrock' ? (
          <div className="grid gap-4 sm:grid-cols-3">
            <Field label="Region" htmlFor="region">
              <Input id="region" value={bedrockRegion} onChange={(e) => setBedrockRegion(e.target.value)} />
            </Field>
            <Field label="Access key id" htmlFor="aws-id">
              <Input id="aws-id" value={bedrockAccess} onChange={(e) => setBedrockAccess(e.target.value)} />
            </Field>
            <Field label="Secret access key" htmlFor="aws-secret">
              <Input id="aws-secret" type="password" autoComplete="off" value={bedrockSecret} onChange={(e) => setBedrockSecret(e.target.value)} />
            </Field>
          </div>
        ) : (
          <Field
            label="API key"
            htmlFor="api-key"
            hint={provider === 'custom' ? 'Sent as x-api-key (Anthropic) or Authorization: Bearer (OpenAI) depending on the URL.' : 'Stored encrypted at rest; never returned through the API.'}
          >
            <Input
              id="api-key"
              type="password"
              autoComplete="off"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={providerDefaults[provider].placeholder}
              mono
            />
          </Field>
        )}
      </div>

      {formError && (
        <div className="mt-4 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-sm text-warning-foreground flex items-start gap-2">
          <ShieldAlert className="mt-0.5 h-4 w-4" />
          <span>{formError}</span>
        </div>
      )}

      <div className="mt-5 flex items-center gap-3">
        <Button variant="outline" onClick={probe} disabled={probeState.kind === 'probing'}>
          {probeState.kind === 'probing' ? <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" /> : <PlugZap className="h-3.5 w-3.5 mr-1.5" />}
          Test connection
        </Button>
        {probeState.kind === 'ok' && (
          <span className="text-xs text-success flex items-center gap-1.5">
            <CheckCircle2 className="h-3.5 w-3.5" />
            Connected · {probeState.latencyMs}ms · {probeState.model}
          </span>
        )}
        {probeState.kind === 'error' && (
          <span className="text-xs text-destructive">{probeState.message}</span>
        )}
      </div>

      <Footer
        back={{ onClick: onBack, label: 'Back' }}
        next={{ disabled, loading: save.isPending, label: 'Continue', onClick: () => save.mutate() }}
      />
    </section>
  );
}

function Finish({ onDone }: { onDone: () => void }) {
  return (
    <section className="text-center">
      <div className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-success/15 text-success">
        <CheckCircle2 className="h-6 w-6" />
      </div>
      <h2 className="mt-5 text-2xl font-semibold tracking-tight text-text-strong">
        Workspace ready.
      </h2>
      <p className="mt-2 text-text-muted max-w-md mx-auto">
        Your organisation and provider are configured. You can author capabilities
        and route them through review before activating.
      </p>
      <Button size="lg" onClick={onDone} className="mt-7">
        Open the control plane
        <ArrowRight className="ml-1.5 h-4 w-4" />
      </Button>
    </section>
  );
}

function Header({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-5">
      <h2 className="text-xl font-semibold tracking-tight text-text-strong">{title}</h2>
      <p className="mt-1 text-sm text-text-muted">{subtitle}</p>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  hint,
  children,
}: {
  label: string;
  htmlFor: string;
  hint?: string | undefined;
  children: React.ReactNode;
}) {
  return (
    <div>
      <Label htmlFor={htmlFor} className="text-xs uppercase tracking-wider text-text-subtle">{label}</Label>
      <div className="mt-2">{children}</div>
      {hint && <div className="mt-1.5 text-xs text-text-subtle">{hint}</div>}
    </div>
  );
}

function Banner({ tone, message }: { tone: 'error' | 'warn'; message: string }) {
  return (
    <div
      className={cn(
        'mt-4 flex items-start gap-2 rounded-md border px-3 py-2 text-sm',
        tone === 'error' && 'border-destructive/40 bg-destructive/10 text-destructive',
        tone === 'warn' && 'border-warning/40 bg-warning/10 text-warning',
      )}
    >
      <ShieldAlert className="mt-0.5 h-4 w-4" />
      <span>{message}</span>
    </div>
  );
}

function Footer({
  back,
  next,
}: {
  back?: { onClick: () => void; label: string } | undefined;
  next: { onClick: () => void; label: string; loading?: boolean | undefined; disabled?: boolean | undefined };
}) {
  return (
    <div className="mt-8 flex items-center justify-between">
      {back ? (
        <Button variant="ghost" onClick={back.onClick}>
          <ArrowLeft className="mr-1.5 h-3.5 w-3.5" /> {back.label}
        </Button>
      ) : (
        <span />
      )}
      <Button onClick={next.onClick} disabled={next.disabled}>
        {next.loading && <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" />}
        {next.label}
        <ArrowRight className="ml-1.5 h-3.5 w-3.5" />
      </Button>
    </div>
  );
}
