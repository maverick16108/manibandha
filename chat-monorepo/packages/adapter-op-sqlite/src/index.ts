// Реализация DbAdapter поверх op-sqlite (React Native). Персистентная нативная SQLite.
import { open, type DB } from '@op-engineering/op-sqlite';
import { SCHEMA_SQL, MIGRATIONS, schemaStatements, type DbAdapter, type DbChange } from '@manibandha/chat-core';

export class OpSqliteAdapter implements DbAdapter {
  private db: DB | null = null;
  private subs = new Set<(e: DbChange) => void>();

  constructor(private name = 'manibandha-chat') {}

  async init(): Promise<void> {
    this.db = open({ name: this.name });
    // op-sqlite.execute выполняет один оператор — разбиваем схему
    for (const stmt of schemaStatements()) await this.db.execute(stmt);
    for (const m of MIGRATIONS) {
      try { await this.db.execute(m); } catch { /* колонка уже есть */ }
    }
  }

  private emit(tables: string[]): void {
    if (!tables || !tables.length) return; // «тихие» записи не будят UI
    for (const cb of this.subs) { try { cb({ tables }); } catch { /* ignore */ } }
  }

  async exec(sql: string): Promise<void> {
    if (!this.db) throw new Error('db not initialized');
    for (const stmt of sql.split(';').map((s) => s.trim()).filter(Boolean)) await this.db.execute(stmt);
  }

  async run(sql: string, params: unknown[] = [], tables: string[] = []): Promise<{ changes: number }> {
    if (!this.db) throw new Error('db not initialized');
    const r = await this.db.execute(sql, params as any[]);
    this.emit(tables);
    return { changes: r.rowsAffected ?? 0 };
  }

  async all<T = Record<string, unknown>>(sql: string, params: unknown[] = []): Promise<T[]> {
    if (!this.db) throw new Error('db not initialized');
    const r = await this.db.execute(sql, params as any[]);
    return (r.rows?._array ?? []) as T[];
  }

  async get<T = Record<string, unknown>>(sql: string, params: unknown[] = []): Promise<T | null> {
    const rows = await this.all<T>(sql, params);
    return rows[0] ?? null;
  }

  async batch(items: { sql: string; params?: unknown[] }[], tables: string[] = []): Promise<void> {
    if (!this.db) throw new Error('db not initialized');
    await this.db.execute('BEGIN');
    try {
      for (const it of items) await this.db.execute(it.sql, (it.params ?? []) as any[]);
      await this.db.execute('COMMIT');
    } catch (e) {
      try { await this.db.execute('ROLLBACK'); } catch { /* ignore */ }
      throw e;
    }
    this.emit(tables);
  }

  subscribe(cb: (e: DbChange) => void): () => void {
    this.subs.add(cb);
    return () => this.subs.delete(cb);
  }

  async close(): Promise<void> {
    try { this.db?.close(); } catch { /* ignore */ }
    this.db = null;
  }
}
