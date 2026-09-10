export interface VaultEntry {
  id: string;
  workspaceId: string;
  key: string;
  valueCiphertext: Buffer | null;
  valueHash: string;
  kind: string;
  createdAt: string;
  updatedAt: string;
}
