import type Database from 'better-sqlite3';
import type { CapabilityContract } from '@promptsheon/shared';

export class CapabilityContractRepo {
  constructor(private db: Database.Database) {}

  getByCapabilityId(capabilityId: string): CapabilityContract | null {
    return this.db.prepare('SELECT * FROM capability_contracts WHERE capability_id = ?')
      .get(capabilityId) as CapabilityContract | null;
  }

  upsert(contract: CapabilityContract): void {
    const existing = this.getByCapabilityId(contract.capabilityId);
    const now = new Date().toISOString();
    if (existing) {
      this.db.prepare(`
        UPDATE capability_contracts SET blast_radius = ?, success_rubric = ?, auto_promotable = ?,
          input_schema = ?, output_schema = ?, slo_max_p95_ms = ?, slo_min_success = ?, slo_max_hallu = ?, updated_at = ?
        WHERE capability_id = ?
      `).run(contract.blastRadius, contract.successRubric, contract.autoPromotable ? 1 : 0, contract.inputSchema, contract.outputSchema, contract.sloMaxP95Ms, contract.sloMinSuccess, contract.sloMaxHallu, now, contract.capabilityId);
    } else {
      this.db.prepare(`
        INSERT INTO capability_contracts (capability_id, blast_radius, success_rubric, auto_promotable, input_schema, output_schema, slo_max_p95_ms, slo_min_success, slo_max_hallu, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      `).run(contract.capabilityId, contract.blastRadius, contract.successRubric, contract.autoPromotable ? 1 : 0, contract.inputSchema, contract.outputSchema, contract.sloMaxP95Ms, contract.sloMinSuccess, contract.sloMaxHallu, now);
    }
  }

  delete(capabilityId: string): void {
    this.db.prepare('DELETE FROM capability_contracts WHERE capability_id = ?').run(capabilityId);
  }
}
