'use client';

import { useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, GitBranch } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Field, FieldGroup } from '@/components/brand/field';
import { repoApi } from '@/lib/api';

const CreateRepositorySchema = z.object({
  workspaceId: z.string().uuid({ message: 'pick a workspace' }),
  name: z.string().min(1, 'name is required').max(120),
  slug: z
    .string()
    .max(60)
    .regex(/^[a-z0-9-]+$/, { message: 'lowercase letters, digits, hyphens only' })
    .optional(),
  description: z.string().max(500).optional(),
  defaultBranch: z.string().min(1).max(60).optional(),
  visibility: z.enum(['private', 'internal', 'public']).default('private'),
  minApprovers: z.coerce.number().int().min(0).max(10).default(1),
  requireSignedReleases: z.boolean().default(false),
});

type FormValues = z.input<typeof CreateRepositorySchema>;
type CreateRepositoryInput = z.input<typeof CreateRepositorySchema>;
type CreateRepositoryOutput = z.output<typeof CreateRepositorySchema>;
// Keep the names exported even though RHF's form uses the input
// form and api.ts uses the output form (because of .default()).
export type { CreateRepositoryInput, CreateRepositoryOutput };

type CreateRepositoryFormValues = z.infer<typeof CreateRepositorySchema>;

export interface NewRepositoryDialogProps {
  workspaceId?: string;
  workspaces: Array<{ id: string; name: string }>;
  disabled?: boolean;
}

export function NewRepositoryDialog({ workspaceId, workspaces, disabled }: NewRepositoryDialogProps) {
  const [open, setOpen] = useState(false);
  const qc = useQueryClient();
  const form = useForm<FormValues>({
    resolver: zodResolver(CreateRepositorySchema),
    defaultValues: {
      workspaceId: workspaceId ?? workspaces[0]?.id ?? '',
      name: '',
      slug: '',
      description: '',
      defaultBranch: 'main',
      visibility: 'private',
      minApprovers: 1,
      requireSignedReleases: false,
    },
  });

  const onSubmit = form.handleSubmit(async (data) => {
    const output = CreateRepositorySchema.parse(data) as CreateRepositoryOutput;
    const payload: Parameters<typeof repoApi.create>[0] = {
      workspaceId: output.workspaceId,
      name: output.name,
    };
    if (output.slug) payload.slug = output.slug;
    if (output.description) payload.description = output.description;
    if (output.defaultBranch) payload.defaultBranch = output.defaultBranch;
    if (output.visibility) payload.visibility = output.visibility;
    if (typeof output.minApprovers === 'number') payload.minApprovers = output.minApprovers;
    if (typeof output.requireSignedReleases === 'boolean')
      payload.requireSignedReleases = output.requireSignedReleases;
    await repoApi.create(payload);
    await qc.invalidateQueries({ queryKey: ['repos'] });
    setOpen(false);
    form.reset();
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button disabled={disabled}>
          <GitBranch className="mr-1.5 h-3.5 w-3.5" />
          New repository
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Create a repository</DialogTitle>
          <DialogDescription>
            Repositories hold branches, tags, commits, and merge requests — and they gate releases.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-5">
          <FieldGroup>
            {workspaces.length > 1 && (
              <Field
                label="Workspace"
                htmlFor="repo-workspace"
                error={form.formState.errors.workspaceId?.message}
              >
                <ThemedSelect
                  value={form.watch('workspaceId')}
                  onValueChange={(v) => form.setValue('workspaceId', v, { shouldValidate: true })}
                  options={workspaces.map((w) => ({ value: w.id, label: w.name }))}
                />
              </Field>
            )}
            <Field label="Name" htmlFor="repo-name" error={form.formState.errors.name?.message} required>
              <Input id="repo-name" {...form.register('name')} placeholder="capability-prompts" />
            </Field>
            <Field label="Slug" htmlFor="repo-slug" hint="lowercase letters, digits, hyphens" error={form.formState.errors.slug?.message}>
              <Input id="repo-slug" {...form.register('slug')} placeholder="capability-prompts" />
            </Field>
            <Field label="Description" htmlFor="repo-desc" error={form.formState.errors.description?.message}>
              <Input id="repo-desc" {...form.register('description')} placeholder="What lives here?" />
            </Field>
            <Field label="Default branch" htmlFor="repo-default-branch" error={form.formState.errors.defaultBranch?.message}>
              <Input id="repo-default-branch" {...form.register('defaultBranch')} />
            </Field>
            <Field label="Visibility" htmlFor="repo-visibility" error={form.formState.errors.visibility?.message}>
              <ThemedSelect
                value={form.watch('visibility') ?? 'private'}
                onValueChange={(v) => form.setValue('visibility', v as 'private' | 'internal' | 'public', { shouldValidate: true })}
                options={[
                  { value: 'private', label: 'Private (default)' },
                  { value: 'internal', label: 'Internal' },
                  { value: 'public', label: 'Public' },
                ]}
              />
            </Field>
            <Field
              label="Min approvers"
              htmlFor="repo-min-approvers"
              hint="Distinct approvers required to merge (0–10)"
              error={form.formState.errors.minApprovers?.message}
            >
              <Input
                id="repo-min-approvers"
                type="number"
                min={0}
                max={10}
                {...form.register('minApprovers', { valueAsNumber: true })}
              />
            </Field>
          </FieldGroup>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              {...form.register('requireSignedReleases')}
              className="h-4 w-4 rounded border-border-subtle"
            />
            <span>Require signed releases</span>
          </label>
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
                'Create repository'
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
