export interface ProviderKey {
  id: string;
  providerName: string;
  keyName: string;
  encryptedKey: Buffer;
  createdAt: string;
  updatedAt: string;
  rotatedAt: string | null;
  lastUsedAt: string | null;
}
