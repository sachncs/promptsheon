import type { Workspace } from '@promptsheon/shared';
import type { Paginated } from '../repos/base.js';

/** Persistence operations required by the workspace use cases. */
export interface WorkspaceStore {
  findById(id: string): Workspace | null;
  findMany(options: { page: number; pageSize: number }): Paginated<Workspace>;
  create(input: { name: string; organization?: string }): Workspace;
  update(id: string, input: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null;
  delete(id: string): boolean;
}

/** Application service for workspace lifecycle operations. */
export class WorkspaceService {
  constructor(private readonly store: WorkspaceStore) {}

  list(options: { page: number; pageSize: number }): Paginated<Workspace> {
    return this.store.findMany(options);
  }

  get(id: string): Workspace | null {
    return this.store.findById(id);
  }

  create(input: { name: string; organization?: string }): Workspace {
    return this.store.create(input);
  }

  update(id: string, input: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null {
    return this.store.update(id, input);
  }

  remove(id: string): boolean {
    return this.store.delete(id);
  }
}
