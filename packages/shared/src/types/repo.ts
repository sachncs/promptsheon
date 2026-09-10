/**
 * Repository — a workspace-scoped Git-like store of files, branches,
 * tags, and commits. The compilation unit for a capability is the
 * tree at a particular ref on a particular repository.
 *
 * Repositories are an alias for the historical "project" concept.
 * Old rows in the `projects` table are auto-promoted into
 * `repositories` by migration 027, preserving the same ids.
 */

export interface Repository {
  id: string;
  workspaceId: string;
  name: string;
  slug: string;
  description: string | null;
  defaultBranch: string;
  visibility: 'private' | 'internal' | 'public';
  minApprovers: number;
  requireSignedReleases: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface RepositoryCreateInput {
  workspaceId: string;
  name: string;
  slug?: string;
  description?: string | null;
  defaultBranch?: string;
  visibility?: Repository['visibility'];
  minApprovers?: number;
  requireSignedReleases?: boolean;
}

export interface RepositoryUpdateInput {
  name?: string;
  description?: string | null;
  defaultBranch?: string;
  visibility?: Repository['visibility'];
  minApprovers?: number;
  requireSignedReleases?: boolean;
}

export const RepositoryVisibility = ['private', 'internal', 'public'] as const;
