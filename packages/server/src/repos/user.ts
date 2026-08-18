import type Database from 'better-sqlite3';
import type { User } from '@promptsheon/shared';
import { notFound, conflict } from '@promptsheon/shared';

export class UserRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): User | null {
    return this.db.prepare('SELECT * FROM users WHERE id = ?').get(id) as User | null;
  }

  findByEmail(email: string): User | null {
    return this.db.prepare('SELECT * FROM users WHERE email = ?').get(email) as User | null;
  }

  findMany(): User[] {
    return this.db.prepare('SELECT * FROM users ORDER BY created_at DESC').all() as User[];
  }

  create(data: { email: string; name: string; role?: string }): User {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.email, data.name, data.role ?? 'reader', now, now);
    return { id, email: data.email, name: data.name, role: (data.role ?? 'reader') as User['role'], createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<User, 'name' | 'role'>>): User {
    const existing = this.findById(id);
    if (!existing) throw notFound('user', id);
    const now = new Date().toISOString();
    this.db.prepare('UPDATE users SET name = ?, role = ?, updated_at = ? WHERE id = ?')
      .run(data.name ?? existing.name, data.role ?? existing.role, now, id);
    return { ...existing, ...data, updatedAt: now };
  }

  delete(id: string): void {
    const existing = this.findById(id);
    if (!existing) throw notFound('user', id);
    this.db.prepare('DELETE FROM users WHERE id = ?').run(id);
  }
}
