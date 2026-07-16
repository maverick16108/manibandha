// Главный поток: адаптер поверх воркера с wa-sqlite/OPFS (RPC через postMessage).
export class WaSqliteAdapter {
  constructor(name = 'manibandha-chat') {
    this.name = name;
    this._subs = new Set();
    this._pending = new Map();
    this._nextId = 1;
    this.worker = null;
  }

  async init() {
    this.worker = new Worker(new URL('./wa-sqlite-worker.js', import.meta.url), { type: 'module' });
    this.worker.onmessage = (e) => {
      const { id, result, error } = e.data;
      const p = this._pending.get(id);
      if (!p) return;
      this._pending.delete(id);
      if (error) p.reject(new Error(error));
      else p.resolve(result);
    };
    this.worker.onerror = (ev) => {
      const err = new Error('worker error: ' + (ev?.message || 'unknown'));
      for (const p of this._pending.values()) p.reject(err);
      this._pending.clear();
    };
    await this._call('init', { name: this.name });
    return this;
  }

  _call(op, extra) {
    return new Promise((resolve, reject) => {
      const id = this._nextId++;
      this._pending.set(id, { resolve, reject });
      this.worker.postMessage({ id, op, ...extra });
    });
  }

  subscribe(cb) {
    this._subs.add(cb);
    return () => this._subs.delete(cb);
  }

  _emit(tables = []) {
    if (!tables || !tables.length) return; // «тихие» записи не будят UI
    for (const cb of this._subs) {
      try { cb({ tables }); } catch { /* ignore */ }
    }
  }

  async exec(sql) { await this._call('exec', { sql }); }

  async run(sql, params = [], tables = []) {
    const r = await this._call('run', { sql, params });
    this._emit(tables);
    return r;
  }

  async all(sql, params = []) { return this._call('all', { sql, params }); }

  async get(sql, params = []) { return this._call('get', { sql, params }); }

  async batch(items = [], tables = []) {
    await this._call('batch', { items });
    this._emit(tables);
  }

  async close() {
    try { this.worker?.terminate(); } catch { /* ignore */ }
    this.worker = null;
  }
}
