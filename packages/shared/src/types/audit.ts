export interface AuditEntry {
  id: string;
  userId: string;
  action: string;
  resource: string;
  details: string;
  timestamp: string;
  previousHash: string;
  entryHash: string;
  timestampStr: string;
  resourceKind: string;
  resourceId: string;
}
