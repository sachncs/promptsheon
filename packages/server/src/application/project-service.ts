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
  findByIdInOrg(id: string, organizationId: string): Project | null;
  findManyInOrg(organizationId: string, options: { page: number; pageSize: number }): Paginated<Project>;
  findByWorkspaceIdInOrg(workspaceId: string, organizationId: string): Project[];
  createInOrg(input: { workspaceId: string; name: string; description?: string }, organizationId: string): Project | null;
  updateInOrg(id: string, organizationId: string, input: Partial<Pick<Project, 'name' | 'description'>>): Project | null;
  deleteInOrg(id: string, organizationId: string): boolean;
}

/** Application service for project lifecycle operations. */
export class ProjectService {
  constructor(private readonly store: ProjectStore) {}

  listByWorkspace(workspaceId: string, organizationId: string): Project[] {
    return this.store.findByWorkspaceIdInOrg(workspaceId, organizationId);
  }

  list(organizationId: string, options: { page: number; pageSize: number }): Paginated<Project> {
    return this.store.findManyInOrg(organizationId, options);
  }

  get(id: string, organizationId: string): Project | null {
    return this.store.findByIdInOrg(id, organizationId);
  }

  create(input: { workspaceId: string; name: string; description?: string }, organizationId: string): Project | null {
    return this.store.createInOrg(input, organizationId);
  }

  update(id: string, organizationId: string, input: Partial<Pick<Project, 'name' | 'description'>>): Project | null {
    return this.store.updateInOrg(id, organizationId, input);
  }

  remove(id: string, organizationId: string): boolean {
    return this.store.deleteInOrg(id, organizationId);
  }
}
