import type Database from 'better-sqlite3';

export interface OrgSettings {
  orgId: string;
  residency: 'local' | 'us' | 'eu' | 'ap' | 'sa' | 'me' | 'af';
  encryptionAtRest: boolean;
  kmsProvider: 'local' | 'aws-sm' | 'hashicorp-vault' | 'doppler';
}

interface Row {
  id: string;
  name: string;
  slug: string;
  residency: 'local' | 'us' | 'eu' | 'ap' | 'sa' | 'me' | 'af';
  encryption_at_rest: number;
  kms_provider: 'local' | 'aws-sm' | 'hashicorp-vault' | 'doppler';
}

const RESIDENCY: OrgSettings['residency'][] = ['local', 'us', 'eu', 'ap', 'sa', 'me', 'af'];
const KMS: OrgSettings['kmsProvider'][] = ['local', 'aws-sm', 'hashicorp-vault', 'doppler'];

export class OrgSettingsRepo {
  constructor(private db: Database.Database) {}

  get(orgId: string): OrgSettings | null {
    const row = this.db
      .prepare('SELECT * FROM orgs WHERE id = ?')
      .get(orgId) as Row | undefined;
    if (!row) return null;
    return {
      orgId: row.id,
      residency: row.residency,
      encryptionAtRest: row.encryption_at_rest === 1,
      kmsProvider: row.kms_provider,
    };
  }

  update(
    orgId: string,
    patch: Partial<Pick<OrgSettings, 'residency' | 'encryptionAtRest' | 'kmsProvider'>>,
  ): OrgSettings | null {
    const current = this.get(orgId);
    if (!current) return null;
    const next: OrgSettings = {
      ...current,
      ...(patch.residency !== undefined ? { residency: patch.residency } : {}),
      ...(patch.encryptionAtRest !== undefined ? { encryptionAtRest: patch.encryptionAtRest } : {}),
      ...(patch.kmsProvider !== undefined ? { kmsProvider: patch.kmsProvider } : {}),
    };
    if (patch.residency !== undefined && !RESIDENCY.includes(patch.residency)) {
      throw new Error(`invalid residency: ${patch.residency}`);
    }
    if (patch.kmsProvider !== undefined && !KMS.includes(patch.kmsProvider)) {
      throw new Error(`invalid kms provider: ${patch.kmsProvider}`);
    }
    this.db
      .prepare(
        'UPDATE orgs SET residency = ?, encryption_at_rest = ?, kms_provider = ? WHERE id = ?',
      )
      .run(next.residency, next.encryptionAtRest ? 1 : 0, next.kmsProvider, orgId);
    return next;
  }
}
