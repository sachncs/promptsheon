import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import {
  buildSignificanceReport,
  type SignificanceReport,
  type VariantStats,
} from '../analysis/significance.js';

export interface ExperimentVariant {
  id: string;
  releaseId: string;
  label: string;
  config: string;
  weight: number;
  createdAt: string;
}

export interface ExperimentAssignment {
  id: string;
  experimentId: string;
  caseId: string;
  variantId: string;
  outcome: string;
  createdAt: string;
}

interface VariantRow {
  id: string;
  release_id: string;
  label: string;
  config: string;
  weight: number;
  created_at: string;
}

interface AssignmentRow {
  id: string;
  experiment_id: string;
  case_id: string;
  variant_id: string;
  outcome: string;
  created_at: string;
}

function toVariant(r: VariantRow): ExperimentVariant {
  return {
    id: r.id,
    releaseId: r.release_id,
    label: r.label,
    config: r.config,
    weight: r.weight,
    createdAt: r.created_at,
  };
}

function toAssignment(r: AssignmentRow): ExperimentAssignment {
  return {
    id: r.id,
    experimentId: r.experiment_id,
    caseId: r.case_id,
    variantId: r.variant_id,
    outcome: r.outcome,
    createdAt: r.created_at,
  };
}

export class ExperimentRepo {
  constructor(private db: Database.Database) {}

  listVariants(releaseId: string): ExperimentVariant[] {
    const rows = this.db
      .prepare('SELECT * FROM experiment_variants WHERE release_id = ? ORDER BY created_at ASC')
      .all(releaseId) as VariantRow[];
    return rows.map(toVariant);
  }

  findVariant(id: string): ExperimentVariant | null {
    const row = this.db
      .prepare('SELECT * FROM experiment_variants WHERE id = ?')
      .get(id) as VariantRow | undefined;
    return row ? toVariant(row) : null;
  }

  createVariant(input: {
    releaseId: string;
    label: string;
    config: string;
    weight: number;
  }): ExperimentVariant {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO experiment_variants (id, release_id, label, config, weight)
         VALUES (?, ?, ?, ?, ?)`,
      )
      .run(id, input.releaseId, input.label, input.config, input.weight);
    return this.findVariant(id)!;
  }

  recordAssignment(input: {
    experimentId: string;
    caseId: string;
    variantId: string;
    outcome: string;
  }): ExperimentAssignment {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO experiment_assignments (id, experiment_id, case_id, variant_id, outcome)
         VALUES (?, ?, ?, ?, ?)`,
      )
      .run(id, input.experimentId, input.caseId, input.variantId, input.outcome);
    return {
      id,
      experimentId: input.experimentId,
      caseId: input.caseId,
      variantId: input.variantId,
      outcome: input.outcome,
      createdAt: new Date().toISOString(),
    };
  }

  listAssignments(experimentId: string): ExperimentAssignment[] {
    const rows = this.db
      .prepare('SELECT * FROM experiment_assignments WHERE experiment_id = ? ORDER BY created_at ASC')
      .all(experimentId) as AssignmentRow[];
    return rows.map(toAssignment);
  }

  /**
   * Side-by-side comparison of variants: returns the pass-rate
   * (fraction passed) for each label in a single map.
   */
  compare(releaseId: string): Array<{ label: string; cases: number; passRate: number; weight: number }> {
    const variants = this.listVariants(releaseId);
    return variants.map((v) => {
      const assignments = this.listAssignments(v.id);
      const passes = assignments.filter((a) => a.outcome === 'pass').length;
      const total = assignments.length;
      return {
        label: v.label,
        cases: total,
        passRate: total > 0 ? passes / total : 0,
        weight: v.weight,
      };
    });
  }

  /**
   * Per-variant counts in the shape the significance module
   * expects: label, passes, fails, total, pass-rate.
   */
  variantStats(releaseId: string): VariantStats[] {
    return this.listVariants(releaseId).map((v) => {
      const assignments = this.listAssignments(v.id);
      const passes = assignments.filter((a) => a.outcome === 'pass').length;
      const fails = assignments.filter((a) => a.outcome === 'fail').length;
      const cases = assignments.length;
      return {
        label: v.label,
        cases,
        passes,
        fails,
        passRate: cases > 0 ? passes / cases : 0,
      };
    });
  }

  /**
   * Statistical-significance summary for a release's variants:
   * pairwise two-proportion z-tests + Bayesian beta-binomial
   * Monte Carlo, plus a winner verdict when at least one pair is
   * significant at the supplied alpha.
   *
   * Returns null when no variant has any observations.
   */
  summarize(
    releaseId: string,
    options: { alpha?: number; bayesSamples?: number } = {},
  ): SignificanceReport | null {
    return buildSignificanceReport(this.variantStats(releaseId), options);
  }
}
