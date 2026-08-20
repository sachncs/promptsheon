import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type {
  Repository,
  RepositoryCreateInput,
  RepositoryUpdateInput,
} from '@promptsheon/shared';
import { BaseRepo } from './base.js';

interface RepositoryRow {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  description: string | null;
  default_branch: string;
  visibility: Repository['visibility'];
  min_approvers: number;
  require_signed_releases: number;
  created_at: string;
  updated_at: string;
}

function toRepository(row: RepositoryRow): Repository {
  return {
    id: row.id,
    workspaceId: row.workspace_id,
    name: row.name,
    slug: row.slug,
    description: row.description,
    defaultBranch: row.default_branch,
    visibility: row.visibility,
    minApprovers: row.min_approvers,
    requireSignedReleases: row.require_signed_releases === 1,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

export class RepoRepo extends BaseRepo<Repository> {
  constructor(db: Database.Database) {
    super(db, 'repositories');
  }

  listByWorkspace(workspaceId: string): Repository[] {
    const rows = this.db
      .prepare('SELECT * FROM repositories WHERE workspace_id = ? ORDER BY created_at ASC')
      .all(workspaceId) as RepositoryRow[];
    return rows.map(toRepository);
  }

  findByWorkspaceAndSlug(workspaceId: string, slug: string): Repository | null {
    const row = this.db
      .prepare('SELECT * FROM repositories WHERE workspace_id = ? AND slug = ?')
      .get(workspaceId, slug) as RepositoryRow | undefined;
    return row ? toRepository(row) : null;
  }

  create(input: RepositoryCreateInput): Repository {
    const id = randomUUID();
    const now = new Date().toISOString();
    const slug = input.slug ?? slugify(input.name);
    this.db
      .prepare(
        `INSERT INTO repositories (
            id, workspace_id, name, slug, description,
            default_branch, visibility, min_approvers, require_signed_releases,
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.workspaceId,
        input.name,
        slug,
        input.description ?? null,
        input.defaultBranch ?? 'main',
        input.visibility ?? 'private',
        input.minApprovers ?? 1,
        input.requireSignedReleases ? 1 : 0,
        now,
        now,
      );
    return this.findById(id)!;
  }

  update(id: string, patch: RepositoryUpdateInput): Repository | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const next = {
      ...existing,
      ...(patch.name !== undefined ? { name: patch.name } : {}),
      ...(patch.description !== undefined ? { description: patch.description } : {}),
      ...(patch.defaultBranch !== undefined ? { defaultBranch: patch.defaultBranch } : {}),
      ...(patch.visibility !== undefined ? { visibility: patch.visibility } : {}),
      ...(patch.minApprovers !== undefined ? { minApprovers: patch.minApprovers } : {}),
      ...(patch.requireSignedReleases !== undefined ? { requireSignedReleases: patch.requireSignedReleases } : {}),
      updatedAt: new Date().toISOString(),
    };
    this.db
      .prepare(
        `UPDATE repositories SET
            name = ?, description = ?, default_branch = ?,
            visibility = ?, min_approvers = ?, require_signed_releases = ?,
            updated_at = ?
         WHERE id = ?`,
      )
      .run(
        next.name,
        next.description,
        next.defaultBranch,
        next.visibility,
        next.minApprovers,
        next.requireSignedReleases ? 1 : 0,
        next.updatedAt,
        id,
      );
    return this.findById(id);
  }

  /** Backed by BaseRepo.findById. */
  override findById(id: string): Repository | null {
    const row = this.db
      .prepare('SELECT * FROM repositories WHERE id = ?')
      .get(id) as RepositoryRow | undefined;
    return row ? toRepository(row) : null;
  }
}

function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-')
    .slice(0, 60);
}
