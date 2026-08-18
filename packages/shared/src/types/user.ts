export type UserRole = 'admin' | 'editor' | 'reader' | 'system';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  createdAt: string;
  updatedAt: string;
}
