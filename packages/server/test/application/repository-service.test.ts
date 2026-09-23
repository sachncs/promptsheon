import { describe, expect, it, vi } from 'vitest';
import { RepositoryService, type RepositoryStore } from '../../src/application/repository-service.js';
import type { Repository } from '@promptsheon/shared';

const repository: Repository = {
  id: 'repo-1',
  workspaceId: 'workspace-1',
  name: 'Prompts',
  slug: 'prompts',
  description: null,
  defaultBranch: 'main',
  visibility: 'private',
  minApprovers: 1,
  requireSignedReleases: false,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

function store(): RepositoryStore {
  return {
    listByWorkspace: vi.fn(() => [repository]),
    findById: vi.fn(() => repository),
    findByIdInOrg: vi.fn((_id, orgId) => orgId === 'org-a' ? repository : null),
    workspaceBelongsToOrg: vi.fn((_workspaceId, orgId) => orgId === 'org-a'),
    create: vi.fn(() => repository),
    update: vi.fn(() => repository),
  };
}

describe('RepositoryService', () => {
  it('rejects cross-organization reads and workspace listing', () => {
    const persistence = store();
    const service = new RepositoryService(persistence);

    expect(service.get(repository.id, 'org-b')).toBeNull();
    expect(service.listByWorkspace(repository.workspaceId, 'org-b')).toBeNull();
    expect(persistence.findById).not.toHaveBeenCalled();
    expect(persistence.listByWorkspace).not.toHaveBeenCalled();
  });

  it('does not create a repository in a workspace outside the active organization', () => {
    const persistence = store();
    const service = new RepositoryService(persistence);

    expect(service.create({ workspaceId: repository.workspaceId, name: 'Other' }, 'org-b')).toBeNull();
    expect(persistence.create).not.toHaveBeenCalled();
  });

  it('scopes updates before delegating persistence', () => {
    const persistence = store();
    const service = new RepositoryService(persistence);

    expect(service.update(repository.id, { name: 'Renamed' }, 'org-a')).toEqual(repository);
    expect(persistence.update).toHaveBeenCalledWith(repository.id, { name: 'Renamed' });
  });
});
