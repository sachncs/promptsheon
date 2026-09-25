import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type {
  EvalSuite,
  EvalSuiteVersion,
  EvalSuiteRunInput,
  GraderSpec,
} from '@promptsheon/shared';

interface SuiteRow {
  id: string;
  capability_id: string;
  repository_id: string | null;
  name: string;
  description: string | null;
  current_version: number;
  pass_threshold: number;
  borderline_band: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

interface VersionRow {
  id: string;
  suite_id: string;
  version: number;
  grader_config: string;
  pass_threshold: number;
  borderline_band: number;
  k: number;
  n: number;
  notes: string | null;
  created_by: string;
  created_at: string;
}

function toSuite(row: SuiteRow): EvalSuite {
  return {
    id: row.id,
    capabilityId: row.capability_id,
    repositoryId: row.repository_id,
    name: row.name,
    description: row.description,
    currentVersion: row.current_version,
    passThreshold: row.pass_threshold,
    borderlineBand: row.borderline_band,
    createdBy: row.created_by,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

function toVersion(row: VersionRow): EvalSuiteVersion {
  return {
    id: row.id,
    suiteId: row.suite_id,
    version: row.version,
    graderConfig: JSON.parse(row.grader_config) as EvalSuiteVersion['graderConfig'],
    passThreshold: row.pass_threshold,
    borderlineBand: row.borderline_band,
    k: row.k,
    n: row.n,
    notes: row.notes,
    createdBy: row.created_by,
    createdAt: row.created_at,
  };
}

export class EvalSuiteRepo {
  constructor(private db: Database.Database) {}

  private capabilityScope(organizationId: string): string {
    return `
      JOIN capabilities c ON c.id = s.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?`;
  }

  list(capabilityId?: string): EvalSuite[] {
    if (capabilityId) {
      const rows = this.db
        .prepare('SELECT * FROM eval_suites WHERE capability_id = ? ORDER BY created_at DESC')
        .all(capabilityId) as SuiteRow[];
      return rows.map(toSuite);
    }
    const rows = this.db
      .prepare('SELECT * FROM eval_suites ORDER BY created_at DESC')
      .all() as SuiteRow[];
    return rows.map(toSuite);
  }

  listInOrg(organizationId: string, capabilityId?: string): EvalSuite[] {
    const capabilityFilter = capabilityId ? ' AND s.capability_id = ?' : '';
    const params = capabilityId ? [organizationId, capabilityId] : [organizationId];
    const rows = this.db
      .prepare(
        `SELECT s.* FROM eval_suites s
         ${this.capabilityScope(organizationId)}${capabilityFilter}
         ORDER BY s.created_at DESC`,
      )
      .all(...params) as SuiteRow[];
    return rows.map(toSuite);
  }

  listForRepositoryInOrg(repositoryId: string, organizationId: string): EvalSuite[] {
    const rows = this.db
      .prepare(
        `SELECT s.* FROM eval_suites s
         ${this.capabilityScope(organizationId)} AND s.repository_id = ?
         ORDER BY s.created_at DESC`,
      )
      .all(organizationId, repositoryId) as SuiteRow[];
    return rows.map(toSuite);
  }

  findById(id: string): EvalSuite | null {
    const row = this.db
      .prepare('SELECT * FROM eval_suites WHERE id = ?')
      .get(id) as SuiteRow | undefined;
    return row ? toSuite(row) : null;
  }

  findByIdInOrg(id: string, organizationId: string): EvalSuite | null {
    const row = this.db
      .prepare(
        `SELECT s.* FROM eval_suites s
         ${this.capabilityScope(organizationId)} AND s.id = ?`,
      )
      .get(organizationId, id) as SuiteRow | undefined;
    return row ? toSuite(row) : null;
  }

  capabilityBelongsToOrg(capabilityId: string, organizationId: string): boolean {
    const row = this.db
      .prepare(
        `SELECT 1 AS present FROM capabilities c
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE c.id = ? AND w.org_id = ?`,
      )
      .get(capabilityId, organizationId) as { present: number } | undefined;
    return row !== undefined;
  }

  repositoryBelongsToOrg(repositoryId: string, organizationId: string): boolean {
    const row = this.db
      .prepare(
        `SELECT 1 AS present FROM repositories r
         JOIN workspaces w ON w.id = r.workspace_id
         WHERE r.id = ? AND w.org_id = ?`,
      )
      .get(repositoryId, organizationId) as { present: number } | undefined;
    return row !== undefined;
  }

  createInOrg(input: {
    capabilityId: string;
    repositoryId: string | null;
    name: string;
    description: string | null;
    passThreshold: number;
    borderlineBand: number;
    createdBy: string;
    initialGraders: GraderSpec[];
    notes: string | null;
  }, organizationId: string): { suite: EvalSuite; version: EvalSuiteVersion } | null {
    if (!this.capabilityBelongsToOrg(input.capabilityId, organizationId)) return null;
    if (input.repositoryId && !this.repositoryBelongsToOrg(input.repositoryId, organizationId)) return null;
    return this.create(input);
  }

  create(input: {
    capabilityId: string;
    repositoryId: string | null;
    name: string;
    description: string | null;
    passThreshold: number;
    borderlineBand: number;
    createdBy: string;
    initialGraders: GraderSpec[];
    notes: string | null;
  }): { suite: EvalSuite; version: EvalSuiteVersion } {
    const id = randomUUID();
    const versionId = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO eval_suites (id, capability_id, repository_id, name, description,
            current_version, pass_threshold, borderline_band, created_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.capabilityId,
        input.repositoryId,
        input.name,
        input.description,
        1,
        input.passThreshold,
        input.borderlineBand,
        input.createdBy,
        now,
        now,
      );
    this.db
      .prepare(
        `INSERT INTO eval_suite_versions (id, suite_id, version, grader_config,
            pass_threshold, borderline_band, k, n, notes, created_by, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        versionId,
        id,
        1,
        JSON.stringify(input.initialGraders),
        input.passThreshold,
        input.borderlineBand,
        1,
        1,
        input.notes,
        input.createdBy,
        now,
      );
    return {
      suite: this.findById(id)!,
      version: this.findVersionById(versionId)!,
    };
  }

  // Marker to silence unused-import on platforms where GraderSpec is not
  // resolved transitively. The actual usage is via toVersion's
  // JSON.parse roundtrip above.
  private _types: GraderSpec[] = [];

  listVersions(suiteId: string): EvalSuiteVersion[] {
    const rows = this.db
      .prepare(
        'SELECT * FROM eval_suite_versions WHERE suite_id = ? ORDER BY version DESC',
      )
      .all(suiteId) as VersionRow[];
    return rows.map(toVersion);
  }

  findVersionById(id: string): EvalSuiteVersion | null {
    const row = this.db
      .prepare('SELECT * FROM eval_suite_versions WHERE id = ?')
      .get(id) as VersionRow | undefined;
    return row ? toVersion(row) : null;
  }

  findVersionByIdInOrg(id: string, organizationId: string): EvalSuiteVersion | null {
    const row = this.db
      .prepare(
        `SELECT v.* FROM eval_suite_versions v
         JOIN eval_suites s ON s.id = v.suite_id
         ${this.capabilityScope(organizationId)} AND v.id = ?`,
      )
      .get(organizationId, id) as VersionRow | undefined;
    return row ? toVersion(row) : null;
  }

  findVersion(suiteId: string, version: number): EvalSuiteVersion | null {
    const row = this.db
      .prepare('SELECT * FROM eval_suite_versions WHERE suite_id = ? AND version = ?')
      .get(suiteId, version) as VersionRow | undefined;
    return row ? toVersion(row) : null;
  }

  findVersionInOrg(suiteId: string, version: number, organizationId: string): EvalSuiteVersion | null {
    const row = this.db
      .prepare(
        `SELECT v.* FROM eval_suite_versions v
         JOIN eval_suites s ON s.id = v.suite_id
         ${this.capabilityScope(organizationId)} AND s.id = ? AND v.version = ?`,
      )
      .get(organizationId, suiteId, version) as VersionRow | undefined;
    return row ? toVersion(row) : null;
  }
}

interface ReviewRow {
  id: string;
  case_id: string;
  suite_id: string;
  suite_run_id: string | null;
  submitted_at: string;
  reviewer_id: string | null;
  decided_at: string | null;
  decision: 'approve' | 'reject' | null;
  notes: string | null;
}

export class HumanReviewRepo {
  constructor(private db: Database.Database) {}

  listOpen(): ReviewRow[] {
    return this.db
      .prepare(
        'SELECT * FROM human_review_queue WHERE decided_at IS NULL ORDER BY submitted_at ASC',
      )
      .all() as ReviewRow[];
  }

  listOpenInOrg(organizationId: string): ReviewRow[] {
    return this.db
      .prepare(
        `SELECT h.* FROM human_review_queue h
         JOIN eval_suites s ON s.id = h.suite_id
         JOIN capabilities c ON c.id = s.capability_id
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE h.decided_at IS NULL AND w.org_id = ?
         ORDER BY h.submitted_at ASC`,
      )
      .all(organizationId) as ReviewRow[];
  }

  enqueue(caseId: string, suiteId: string, suiteRunId: string | null): ReviewRow {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO human_review_queue (id, case_id, suite_id, suite_run_id)
         VALUES (?, ?, ?, ?)`,
      )
      .run(id, caseId, suiteId, suiteRunId);
    return {
      id,
      case_id: caseId,
      suite_id: suiteId,
      suite_run_id: suiteRunId,
      submitted_at: new Date().toISOString(),
      reviewer_id: null,
      decided_at: null,
      decision: null,
      notes: null,
    };
  }

  decide(id: string, reviewerId: string, decision: 'approve' | 'reject', notes: string | null): ReviewRow | null {
    this.db
      .prepare(
        `UPDATE human_review_queue
         SET reviewer_id = ?, decided_at = ?, decision = ?, notes = ?
         WHERE id = ?`,
      )
      .run(reviewerId, new Date().toISOString(), decision, notes, id);
    return this.findById(id);
  }

  decideInOrg(id: string, organizationId: string, reviewerId: string, decision: 'approve' | 'reject', notes: string | null): ReviewRow | null {
    const result = this.db
      .prepare(
        `UPDATE human_review_queue
         SET reviewer_id = ?, decided_at = ?, decision = ?, notes = ?
         WHERE id = ? AND EXISTS (
           SELECT 1 FROM eval_suites s
           JOIN capabilities c ON c.id = s.capability_id
           JOIN projects p ON p.id = c.project_id
           JOIN workspaces w ON w.id = p.workspace_id
           WHERE s.id = human_review_queue.suite_id AND w.org_id = ?
         )`,
      )
      .run(reviewerId, new Date().toISOString(), decision, notes, id, organizationId);
    return result.changes > 0 ? this.findById(id) : null;
  }

  findById(id: string): ReviewRow | null {
    return (this.db
      .prepare('SELECT * FROM human_review_queue WHERE id = ?')
      .get(id) as ReviewRow | undefined) ?? null;
  }
}

void ({} as unknown as EvalSuiteRunInput);
