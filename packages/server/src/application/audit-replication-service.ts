/** A single audit frame received from the primary audit replicator. */
export interface AuditReplicationFrame {
  rowid: number;
  previousHash: string;
  entry: {
    id: string;
    userId: string;
    action: string;
    resource: string;
    details: string;
    timestamp: string;
    entryHash: string;
    resourceKind: string;
    resourceId: string;
  };
}

/** Persistence boundary for replica audit frames. */
export interface AuditReplicationStore {
  ingest(frame: AuditReplicationFrame): { duplicate: boolean };
}

/** Application boundary for idempotent audit replication. */
export class AuditReplicationService {
  constructor(private readonly store: AuditReplicationStore) {}

  ingest(frame: AuditReplicationFrame): { duplicate: boolean } {
    return this.store.ingest(frame);
  }
}
