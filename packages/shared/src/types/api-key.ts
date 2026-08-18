import type { UserRole } from './user.js';

export interface ApiKey {
  id: string;
  userId: string;
  name: string;
  keyHash: string;
  keyPrefix: string;
  role: UserRole;
  expiresAt: string | null;
  lastUsed: string | null;
  createdAt: string;
  revoked: boolean;
}
