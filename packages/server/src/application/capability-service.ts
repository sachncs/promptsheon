import type { Capability } from '@promptsheon/shared';
import type { Paginated } from '../repos/base.js';

/** Persistence operations required by capability use cases. */
export interface CapabilityStore {
  findById(id: string): Capability | null;
  findMany(options: { page: number; pageSize: number }): Paginated<Capability>;
  findByProjectId(projectId: string): Capability[];
  create(input: { projectId: string; name: string; description?: string }): Capability;
  update(id: string, input: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null;
  delete(id: string): boolean;
  findByIdInOrg(id: string, organizationId: string): Capability | null;
  findManyInOrg(organizationId: string, options: { page: number; pageSize: number }): Paginated<Capability>;
  findByProjectIdInOrg(projectId: string, organizationId: string): Capability[];
  createInOrg(input: { projectId: string; name: string; description?: string }, organizationId: string): Capability | null;
  updateInOrg(id: string, organizationId: string, input: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null;
  deleteInOrg(id: string, organizationId: string): boolean;
}

/** Application service for capability lifecycle operations. */
export class CapabilityService {
  constructor(private readonly store: CapabilityStore) {}

  listByProject(projectId: string, organizationId: string): Capability[] {
    return this.store.findByProjectIdInOrg(projectId, organizationId);
  }

  list(organizationId: string, options: { page: number; pageSize: number }): Paginated<Capability> {
    return this.store.findManyInOrg(organizationId, options);
  }

  get(id: string, organizationId: string): Capability | null {
    return this.store.findByIdInOrg(id, organizationId);
  }

  create(input: { projectId: string; name: string; description?: string }, organizationId: string): Capability | null {
    return this.store.createInOrg(input, organizationId);
  }

  update(id: string, organizationId: string, input: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null {
    return this.store.updateInOrg(id, organizationId, input);
  }

  remove(id: string, organizationId: string): boolean {
    return this.store.deleteInOrg(id, organizationId);
  }
}
