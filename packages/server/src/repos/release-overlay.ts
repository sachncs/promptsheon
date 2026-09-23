import type Database from 'better-sqlite3';

export interface ReleaseOverlay {
  releaseId: string;
  environment: string;
  patch: Record<string, unknown>;
  updatedAt: string;
}

interface ReleaseOverlayRow {
  release_id: string;
  environment: string;
  patch_json: string;
  updated_at: string;
}

function toOverlay(row: ReleaseOverlayRow): ReleaseOverlay {
  let patch: Record<string, unknown> = {};
  try {
    const parsed: unknown = JSON.parse(row.patch_json);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      patch = parsed as Record<string, unknown>;
    }
  } catch {
    patch = {};
  }
  return {
    releaseId: row.release_id,
    environment: row.environment,
    patch,
    updatedAt: row.updated_at,
  };
}

/** Persists release evaluation overlays independently from release lifecycle state. */
export class ReleaseOverlayRepo {
  constructor(private readonly db: Database.Database) {}

  get(releaseId: string, environment: string): ReleaseOverlay | null {
    const row = this.db
      .prepare('SELECT release_id, environment, patch_json, updated_at FROM release_overlays WHERE release_id = ? AND environment = ?')
      .get(releaseId, environment) as ReleaseOverlayRow | undefined;
    return row ? toOverlay(row) : null;
  }

  upsert(releaseId: string, environment: string, patch: Record<string, unknown>): ReleaseOverlay {
    const updatedAt = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO release_overlays (release_id, environment, patch_json, updated_at)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(release_id, environment) DO UPDATE SET
           patch_json = excluded.patch_json,
           updated_at = excluded.updated_at`,
      )
      .run(releaseId, environment, JSON.stringify(patch), updatedAt);
    return { releaseId, environment, patch, updatedAt };
  }
}
