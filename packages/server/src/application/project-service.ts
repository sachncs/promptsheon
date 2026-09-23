import type { Project } from '@promptsheon/shared';
import type { Paginated } from '../repos/base.js';

/** Persistence operations required by the project use cases. */
export interface ProjectStore {
  findById(id: string): Project | null;
  findMany(options: { page: number; pageSize: number }): Paginated<Project>;
  findByWorkspaceId(workspaceId: string): Project[];
  create(input: { workspaceId: string; name: string; description?: string }): Project;
  update(id: string, input: Partial<Pick<Project, 'name' | 'description'>>): Project | null;
  delete(id: string): boolean;
}

/** Application service for project lifecycle operations. */
export class ProjectService {
  constructor(private readonly store: ProjectStore) {}

  listByWorkspace(workspaceId: string): Project[] {
    return this.store.findByWorkspaceId(workspaceId);
  }

  list(options: { page: number; pageSize: number }): Paginated<Project> {
    return this.store.findMany(options);
  }

  get(id: string): Project | null {
    return this.store.findById(id);
  }

  create(input: { workspaceId: string; name: string; description?: string }): Project {
    return this.store.create(input);
  }

  update(id: string, input: Partial<Pick<Project, 'name' | 'description'>>): Project | null {
    return this.store.update(id, input);
  }

  remove(id: string): boolean {
    return this.store.delete(id);
  }
}
