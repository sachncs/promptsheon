import type { User, UserRole } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';

interface UserRow {
  id: string;
  email: string;
  name: string;
  role: string;
  created_at: string;
  updated_at: string;
}

/**
 * UserRepo — minimal CRUD over the `users` table. Users are top-level
 * identities that can hold org memberships via `org_members`.
 */
export class UserRepo {
  constructor(private db: Database.Database) {}

  list(): User[] {
    const rows = this.db
      .prepare('SELECT * FROM users ORDER BY created_at ASC')
      .all() as UserRow[];
    return rows.map(this.toUser);
  }

  findById(id: string): User | null {
    const row = this.db
      .prepare('SELECT * FROM users WHERE id = ?')
      .get(id) as UserRow | undefined;
    return row ? this.toUser(row) : null;
  }

  create(data: { email: string; name: string; role?: UserRole }): User {
    const id = randomUUID();
    const now = new Date().toISOString();
    const role = data.role ?? 'reader';
    this.db
      .prepare(
        'INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)',
      )
      .run(id, data.email, data.name, role, now, now);
    return { id, email: data.email, name: data.name, role, createdAt: now, updatedAt: now };
  }

  updateRole(id: string, role: UserRole): User | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const now = new Date().toISOString();
    this.db
      .prepare('UPDATE users SET role = ?, updated_at = ? WHERE id = ?')
      .run(role, now, id);
    return { ...existing, role, updatedAt: now };
  }

  delete(id: string): boolean {
    const result = this.db.prepare('DELETE FROM users WHERE id = ?').run(id);
    return result.changes > 0;
  }

  private toUser = (row: UserRow): User => ({
    id: row.id,
    email: row.email,
    name: row.name,
    role: row.role as UserRole,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  });
}