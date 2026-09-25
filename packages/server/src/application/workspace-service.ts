import type { Workspace } from '@promptsheon/shared';
import type { Paginated } from '../repos/base.js';

/** Persistence operations required by the workspace use cases. */
export interface WorkspaceStore {
  findById(id: string): Workspace | null;
  findMany(options: { page: number; pageSize: number }): Paginated<Workspace>;
  create(input: { name: string; organization?: string }): Workspace;
  update(id: string, input: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null;
  delete(id: string): boolean;
  findByIdInOrg(id: string, organizationId: string): Workspace | null;
  findManyInOrg(organizationId: string, options: { page: number; pageSize: number }): Paginated<Workspace>;
  createInOrg(input: { name: string; organization?: string }, organizationId: string): Workspace | null;
  updateInOrg(id: string, organizationId: string, input: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null;
  deleteInOrg(id: string, organizationId: string): boolean;
}

/** Application service for workspace lifecycle operations. */
export class WorkspaceService {
  constructor(private readonly store: WorkspaceStore) {}

  list(organizationId: string, options: { page: number; pageSize: number }): Paginated<Workspace> {
    return this.store.findManyInOrg(organizationId, options);
  }

  get(id: string, organizationId: string): Workspace | null {
    return this.store.findByIdInOrg(id, organizationId);
  }

  create(input: { name: string; organization?: string }, organizationId: string): Workspace | null {
    return this.store.createInOrg(input, organizationId);
  }

  update(id: string, organizationId: string, input: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null {
    return this.store.updateInOrg(id, organizationId, input);
  }

  remove(id: string, organizationId: string): boolean {
    return this.store.deleteInOrg(id, organizationId);
  }
}
