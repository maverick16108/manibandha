// sql.js реализация адаптера БД (in-memory + опциональный снапшот в IndexedDB).
// Не содержит Vite-специфичных импортов → используется и в браузере (фолбэк),
// и в Node (юнит-тесты sync-движка). Зависимости (initSqlJs, locateFile) — инъекция.
import { SCHEMA_SQL, MIGRATIONS } from './schema.js';

const IDB_DB = 'manibandha-chat-sqljs';
const IDB_STORE = 'snapshots';

export class SqlJsCore {
  constructor(name = 'manibandha-chat', { initSqlJs, locateFile } = {}) {
    this.name = name;
    this._initSqlJs = initSqlJs;
    this._locateFile = locateFile;
    this._subs = new Set();
    this._persistTimer = null;
    this.db = null;
  }

  async init() {
    const SQL = await this._initSqlJs(this._locateFile ? { locateFile: this._locateFile } : undefined);
    let bytes = null;
    try {
      bytes = await this._loadSnapshot();
    } catch { /* нет снапшота — старт с чистой БД */ }
    this.db = bytes ? new SQL.Database(bytes) : new SQL.Database();
    this.db.run(SCHEMA_SQL);
    for (const m of MIGRATIONS) { try { this.db.run(m); } catch { /* колонка уже есть */ } }
    return this;
  }

  _emit(tables = []) {
    if (!tables || !tables.length) return; // «тихие» записи не будят UI
    for (const cb of this._subs) {
      try { cb({ tables }); } catch { /* подписчик не должен ломать запись */ }
    }
  }

  subscribe(cb) {
    this._subs.add(cb);
    return () => this._subs.delete(cb);
  }

  async exec(sql) {
    this.db.exec(sql);
    this._schedulePersist();
  }

  async run(sql, params = [], tables = []) {
    this.db.run(sql, params);
    const changes = this.db.getRowsModified();
    this._schedulePersist();
    this._emit(tables);
    return { changes };
  }

  async all(sql, params = []) {
    const stmt = this.db.prepare(sql);
    try {
      if (params && params.length) stmt.bind(params);
      const rows = [];
      while (stmt.step()) rows.push(stmt.getAsObject());
      return rows;
    } finally {
      stmt.free();
    }
  }

  async get(sql, params = []) {
    const rows = await this.all(sql, params);
    return rows[0] || null;
  }

  async batch(items = [], tables = []) {
    this.db.run('BEGIN');
    try {
      for (const it of items) this.db.run(it.sql, it.params || []);
      this.db.run('COMMIT');
    } catch (e) {
      try { this.db.run('ROLLBACK'); } catch { /* ignore */ }
      throw e;
    }
    this._schedulePersist();
    this._emit(tables);
  }

  async close() {
    if (this._persistTimer) { clearTimeout(this._persistTimer); this._persistTimer = null; }
    await this._persist();
    try { this.db?.close(); } catch { /* ignore */ }
    this.db = null;
  }

  // ── персистентность фолбэка через IndexedDB (в браузере) ────────────────
  _schedulePersist() {
    if (typeof indexedDB === 'undefined') return; // Node — снапшоты не нужны
    if (this._persistTimer) clearTimeout(this._persistTimer);
    this._persistTimer = setTimeout(() => this._persist().catch(() => {}), 400);
  }

  _idb() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open(IDB_DB, 1);
      req.onupgradeneeded = () => req.result.createObjectStore(IDB_STORE);
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  async _persist() {
    if (typeof indexedDB === 'undefined' || !this.db) return;
    const bytes = this.db.export();
    const idb = await this._idb();
    await new Promise((resolve, reject) => {
      const tx = idb.transaction(IDB_STORE, 'readwrite');
      tx.objectStore(IDB_STORE).put(bytes, this.name);
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
    idb.close();
  }

  async _loadSnapshot() {
    if (typeof indexedDB === 'undefined') return null;
    const idb = await this._idb();
    const bytes = await new Promise((resolve, reject) => {
      const tx = idb.transaction(IDB_STORE, 'readonly');
      const req = tx.objectStore(IDB_STORE).get(this.name);
      req.onsuccess = () => resolve(req.result || null);
      req.onerror = () => reject(req.error);
    });
    idb.close();
    return bytes ? new Uint8Array(bytes) : null;
  }
}
