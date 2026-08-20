import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';

export interface RedteamCase {
  id: string;
  packId: string;
  label: string;
  prompt: string;
  expectedRefusalMatch: string | null;
  severity: 'low' | 'medium' | 'high';
  createdAt: string;
}

export interface RedteamPack {
  id: string;
  name: string;
  description: string | null;
  category: 'injection' | 'jailbreak' | 'exfil';
  createdAt: string;
}

export interface RedteamRun {
  id: string;
  packId: string;
  runAt: string;
  resistance: number;
  results: string;
}

interface PackRow {
  id: string;
  name: string;
  description: string | null;
  category: 'injection' | 'jailbreak' | 'exfil';
  created_at: string;
}

interface CaseRow {
  id: string;
  pack_id: string;
  label: string;
  prompt: string;
  expected_refusal_match: string | null;
  severity: 'low' | 'medium' | 'high';
  created_at: string;
}

interface RunRow {
  id: string;
  pack_id: string;
  run_at: string;
  resistance: number;
  results: string;
}

function toPack(r: PackRow): RedteamPack {
  return {
    id: r.id,
    name: r.name,
    description: r.description,
    category: r.category,
    createdAt: r.created_at,
  };
}

function toCase(r: CaseRow): RedteamCase {
  return {
    id: r.id,
    packId: r.pack_id,
    label: r.label,
    prompt: r.prompt,
    expectedRefusalMatch: r.expected_refusal_match,
    severity: r.severity,
    createdAt: r.created_at,
  };
}

function toRun(r: RunRow): RedteamRun {
  return {
    id: r.id,
    packId: r.pack_id,
    runAt: r.run_at,
    resistance: r.resistance,
    results: r.results,
  };
}

export class RedteamRepo {
  constructor(private db: Database.Database) {}

  listPacks(): RedteamPack[] {
    const rows = this.db
      .prepare('SELECT * FROM redteam_packs ORDER BY created_at ASC')
      .all() as PackRow[];
    return rows.map(toPack);
  }

  findPack(id: string): RedteamPack | null {
    const row = this.db
      .prepare('SELECT * FROM redteam_packs WHERE id = ?')
      .get(id) as PackRow | undefined;
    return row ? toPack(row) : null;
  }

  findPackByName(name: string): RedteamPack | null {
    const row = this.db
      .prepare('SELECT * FROM redteam_packs WHERE name = ?')
      .get(name) as PackRow | undefined;
    return row ? toPack(row) : null;
  }

  insertPack(input: { name: string; description: string | null; category: RedteamPack['category'] }): RedteamPack {
    const id = randomUUID();
    this.db
      .prepare('INSERT INTO redteam_packs (id, name, description, category) VALUES (?, ?, ?, ?)')
      .run(id, input.name, input.description, input.category);
    return this.findPack(id)!;
  }

  listCases(packId: string): RedteamCase[] {
    const rows = this.db
      .prepare('SELECT * FROM redteam_cases WHERE pack_id = ? ORDER BY created_at ASC')
      .all(packId) as CaseRow[];
    return rows.map(toCase);
  }

  insertCase(input: {
    packId: string;
    label: string;
    prompt: string;
    expectedRefusalMatch: string | null;
    severity: RedteamCase['severity'];
  }): RedteamCase {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO redteam_cases (id, pack_id, label, prompt, expected_refusal_match, severity)
         VALUES (?, ?, ?, ?, ?, ?)`,
      )
      .run(id, input.packId, input.label, input.prompt, input.expectedRefusalMatch, input.severity);
    return { ...input, id, createdAt: new Date().toISOString() };
  }

  recordRun(input: { packId: string; resistance: number; results: string }): RedteamRun {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO redteam_runs (id, pack_id, run_at, resistance, results)
         VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?)`,
      )
      .run(id, input.packId, input.resistance, input.results);
    return this.findRun(id)!;
  }

  findRun(id: string): RedteamRun | null {
    const row = this.db
      .prepare('SELECT * FROM redteam_runs WHERE id = ?')
      .get(id) as RunRow | undefined;
    return row ? toRun(row) : null;
  }

  latestRunForPack(packId: string): RedteamRun | null {
    const row = this.db
      .prepare('SELECT * FROM redteam_runs WHERE pack_id = ? ORDER BY run_at DESC LIMIT 1')
      .get(packId) as RunRow | undefined;
    return row ? toRun(row) : null;
  }

  /**
   * Compute the resistance score for a pack: the fraction of cases
   * whose response (parsed from results) was refused per the
   * expected_refusal_match pattern. Lower = worse.
   */
  scorePack(results: Array<{ caseId: string; response: string; resisted: boolean }>, cases: RedteamCase[]): number {
    if (cases.length === 0) return 1;
    const resisted = results.filter((r) => r.resisted).length;
    return resisted / cases.length;
  }
}
