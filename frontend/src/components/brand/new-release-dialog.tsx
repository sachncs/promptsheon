'use client';

import { useMemo, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, GitMerge } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup } from '@/components/brand/field';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Input } from '@/components/ui/input';
import { capabilityApi, releaseApi, versionApi, unwrapList, unwrapFirst } from '@/lib/api';

const CreateReleaseSchema = z.object({
  capabilityId: z.string().uuid({ message: 'pick a capability' }),
  capabilityVersion: z.coerce.number().int().positive({ message: 'pick a version' }),
  environment: z.enum(['dev', 'staging', 'prod']),
  manifest: z.string().min(2, 'manifest required'),
  canaryPercent: z.coerce.number().int().min(0).max(100).optional().default(0),
});

type CreateReleaseInput = z.infer<typeof CreateReleaseSchema>;

interface CapabilitySummary {
  id: string;
  name: string;
}

interface VersionSummary {
  id: string;
  version: number;
}

export interface NewReleaseDialogProps {
  /** Optional pre-selected capability id (deep link from a capability page). */
  capabilityId?: string;
}

export function NewReleaseDialog({ capabilityId }: NewReleaseDialogProps) {
  const [open, setOpen] = useState(false);
  const qc = useQueryClient();
  type FormValues = z.input<typeof CreateReleaseSchema>;
  const form = useForm<FormValues>({
    resolver: zodResolver(CreateReleaseSchema),
    defaultValues: {
      capabilityId: capabilityId ?? '',
      capabilityVersion: 1,
      environment: 'dev',
      manifest: '{"nodes":[],"edges":[]}',
      canaryPercent: 0,
    },
  });

  const capabilities = useQuery<CapabilitySummary[]>({
    queryKey: ['capabilities', 'new-release'],
    queryFn: async () => {
      const allCaps = (await qc.fetchQuery({ queryKey: ['capabilities'], staleTime: 60_000 })) as unknown;
      const list = unwrapList<CapabilitySummary>(allCaps);
      return list;
    },
    enabled: open,
  });
  const capabilityIdValue = form.watch('capabilityId');
  const versions = useQuery<VersionSummary[]>({
    queryKey: ['versions', 'by-cap', capabilityIdValue],
    queryFn: async () => {
      if (!capabilityIdValue) return [];
      const r = await versionApi.list(capabilityIdValue);
      const list = unwrapList<VersionSummary>(r.data);
      return list.length > 0 ? list : [{ id: 'placeholder', version: 1 }];
    },
    enabled: Boolean(capabilityIdValue),
  });
  const fallbackManifest = useMemo(() => '{"nodes":[],"edges":[]}', []);
  const selectedVersion = useMemo(() => {
    const list = versions.data ?? [];
    const wanted = form.watch('capabilityVersion');
    const found = unwrapFirst<VersionSummary>(
      list.find((v) => v.version === wanted) ?? list[0],
    );
    return found;
  }, [versions.data, form]);

  const onSubmit = form.handleSubmit(async (data) => {
    await releaseApi.create({
      capabilityId: data.capabilityId,
      capabilityVersion: data.capabilityVersion,
      capabilityVersionId: selectedVersion?.id ?? null,
      manifest: data.manifest,
      environment: data.environment,
      canaryPercent: data.canaryPercent,
    } as Parameters<typeof releaseApi.create>[0]);
    await qc.invalidateQueries({ queryKey: ['releases'] });
    setOpen(false);
    form.reset();
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <GitMerge className="mr-1.5 h-3.5 w-3.5" />
          New release
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Create a release</DialogTitle>
          <DialogDescription>
            Draft a new release for a capability version. The activation gate requires 2 distinct, non-creator approvers.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-5">
          <FieldGroup>
            <Field label="Capability" htmlFor="rel-cap" error={form.formState.errors.capabilityId?.message} required>
              <ThemedSelect
                value={form.watch('capabilityId') ?? ''}
                onValueChange={(v) => {
                  form.setValue('capabilityId', v, { shouldValidate: true });
                  form.setValue('capabilityVersion', 1, { shouldValidate: true });
                }}
                options={(capabilities.data ?? []).map((c) => ({ value: c.id, label: c.name }))}
                placeholder="Select capability"
              />
            </Field>
            <Field label="Version" htmlFor="rel-ver" error={form.formState.errors.capabilityVersion?.message} required>
              <ThemedSelect
                value={String(form.watch('capabilityVersion') ?? '1')}
                onValueChange={(v) => form.setValue('capabilityVersion', Number(v), { shouldValidate: true })}
                options={(versions.data ?? []).map((v) => ({ value: String(v.version), label: `v${v.version}` }))}
                placeholder="Select version"
              />
            </Field>
            <Field label="Environment" htmlFor="rel-env" error={form.formState.errors.environment?.message} required>
              <ThemedSelect
                value={form.watch('environment') ?? 'dev'}
                onValueChange={(v) =>
                  form.setValue('environment', v as 'dev' | 'staging' | 'prod', { shouldValidate: true })
                }
                options={[
                  { value: 'dev', label: 'Dev' },
                  { value: 'staging', label: 'Staging' },
                  { value: 'prod', label: 'Production' },
                ]}
              />
            </Field>
            <Field label="Manifest JSON" htmlFor="rel-manifest" error={form.formState.errors.manifest?.message} required>
              <textarea
                id="rel-manifest"
                rows={6}
                className="w-full rounded-md border border-border-subtle bg-surface-1 p-2 font-mono text-xs"
                defaultValue={fallbackManifest}
                {...form.register('manifest')}
              />
            </Field>
            <Field
              label="Canary percent"
              htmlFor="rel-canary"
              hint="0 = full rollout on activate"
              error={form.formState.errors.canaryPercent?.message}
            >
              <Input
                id="rel-canary"
                type="number"
                min={0}
                max={100}
                {...form.register('canaryPercent', { valueAsNumber: true })}
              />
            </Field>
          </FieldGroup>
          {form.formState.errors.root && (
            <p className="text-xs text-destructive" role="alert">
              {form.formState.errors.root.message}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? (
                <>
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  Creating…
                </>
              ) : (
                'Create release'
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
