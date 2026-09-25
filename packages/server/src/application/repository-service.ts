import type {
  Repository,
  RepositoryCreateInput,
  RepositoryUpdateInput,
} from '@promptsheon/shared';

/** Persistence boundary required by repository use cases. */
export interface RepositoryStore {
  listByWorkspace(workspaceId: string): Repository[];
  findById(id: string): Repository | null;
  findByIdInOrg(id: string, organizationId: string): Repository | null;
  workspaceBelongsToOrg(workspaceId: string, organizationId: string): boolean;
  create(input: RepositoryCreateInput): Repository;
  update(id: string, input: RepositoryUpdateInput): Repository | null;
}

/** Application boundary for repository lifecycle and tenant access. */
export class RepositoryService {
  constructor(private readonly store: RepositoryStore) {}

  listByWorkspace(workspaceId: string, organizationId?: string): Repository[] | null {
    if (organizationId && !this.store.workspaceBelongsToOrg(workspaceId, organizationId)) return null;
    return this.store.listByWorkspace(workspaceId);
  }

  get(id: string, organizationId?: string): Repository | null {
    return organizationId
      ? this.store.findByIdInOrg(id, organizationId)
      : this.store.findById(id);
  }

  create(input: RepositoryCreateInput, organizationId?: string): Repository | null {
    if (organizationId && !this.store.workspaceBelongsToOrg(input.workspaceId, organizationId)) return null;
    return this.store.create(input);
  }

  update(id: string, input: RepositoryUpdateInput, organizationId?: string): Repository | null {
    if (!this.get(id, organizationId)) return null;
    return this.store.update(id, input);
  }
}
