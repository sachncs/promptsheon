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
}

/** Application service for capability lifecycle operations. */
export class CapabilityService {
  constructor(private readonly store: CapabilityStore) {}

  listByProject(projectId: string): Capability[] {
    return this.store.findByProjectId(projectId);
  }

  list(options: { page: number; pageSize: number }): Paginated<Capability> {
    return this.store.findMany(options);
  }

  get(id: string): Capability | null {
    return this.store.findById(id);
  }

  create(input: { projectId: string; name: string; description?: string }): Capability {
    return this.store.create(input);
  }

  update(id: string, input: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null {
    return this.store.update(id, input);
  }

  remove(id: string): boolean {
    return this.store.delete(id);
  }
}
