// Единый интерфейс локальной БД. За ним — платформенная реализация:
//   web:    wa-sqlite + OPFS (Worker)      mobile: op-sqlite      test/Node: sql.js
// Контракт SQL-уровня, чтобы схема была одна на всех платформах.
export interface DbChange {
  tables: string[];
}

export interface DbAdapter {
  /** Открыть БД и применить схему + миграции. */
  init(): Promise<void>;
  /** Выполнить один или несколько DDL-операторов без параметров (без эмита изменений). */
  exec(sql: string): Promise<void>;
  /** Один оператор записи; при непустом tables эмитит изменение подписчикам. */
  run(sql: string, params?: unknown[], tables?: string[]): Promise<{ changes: number }>;
  /** Выборка строк. */
  all<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T[]>;
  /** Первая строка или null. */
  get<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T | null>;
  /** Атомарная пачка операторов (BEGIN/COMMIT); при непустом tables эмитит изменение. */
  batch(items: { sql: string; params?: unknown[] }[], tables?: string[]): Promise<void>;
  /** Подписка на изменения (после каждой записи с непустым tables). */
  subscribe(cb: (e: DbChange) => void): () => void;
  close(): Promise<void>;
}
