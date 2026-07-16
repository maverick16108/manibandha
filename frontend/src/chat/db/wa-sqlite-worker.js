// Web Worker: хостит wa-sqlite (синхронная сборка) поверх OPFS (AccessHandlePoolVFS).
// OPFS SyncAccessHandle доступен только в Worker — поэтому вся работа с БД здесь.
import SQLiteESMFactory from 'wa-sqlite/dist/wa-sqlite.mjs';
import wasmUrl from 'wa-sqlite/dist/wa-sqlite.wasm?url';
import * as SQLite from 'wa-sqlite';
import { AccessHandlePoolVFS } from 'wa-sqlite/src/examples/AccessHandlePoolVFS.js';
import { SCHEMA_SQL, MIGRATIONS } from './schema.js';

let sqlite3 = null;
let db = null;
const SQLITE_ROW = SQLite.SQLITE_ROW;

async function open(name) {
  const module = await SQLiteESMFactory({ locateFile: () => wasmUrl });
  sqlite3 = SQLite.Factory(module);
  const vfs = new AccessHandlePoolVFS(`/${name}`);
  await vfs.isReady;
  sqlite3.vfs_register(vfs, true);
  db = await sqlite3.open_v2(name);
  await sqlite3.exec(db, SCHEMA_SQL);
  for (const m of MIGRATIONS) { try { await sqlite3.exec(db, m); } catch { /* колонка уже есть */ } }
}

async function all(sql, params) {
  const rows = [];
  for await (const stmt of sqlite3.statements(db, sql)) {
    if (params && params.length) sqlite3.bind_collection(stmt, params);
    const cols = sqlite3.column_names(stmt);
    while ((await sqlite3.step(stmt)) === SQLITE_ROW) {
      const r = sqlite3.row(stmt);
      const o = {};
      // wa-sqlite отдаёт INTEGER как BigInt; приводим к Number, чтобы БД вела
      // себя как sql.js (иначе ломается ре-биндинг id/seq и JSON.stringify).
      cols.forEach((c, i) => { const v = r[i]; o[c] = typeof v === 'bigint' ? Number(v) : v; });
      rows.push(o);
    }
  }
  return rows;
}

async function run(sql, params) {
  for await (const stmt of sqlite3.statements(db, sql)) {
    if (params && params.length) sqlite3.bind_collection(stmt, params);
    while ((await sqlite3.step(stmt)) === SQLITE_ROW) { /* drain */ }
  }
  return { changes: sqlite3.changes(db) };
}

async function batch(items) {
  await sqlite3.exec(db, 'BEGIN');
  try {
    for (const it of items) await run(it.sql, it.params || []);
    await sqlite3.exec(db, 'COMMIT');
  } catch (e) {
    try { await sqlite3.exec(db, 'ROLLBACK'); } catch { /* ignore */ }
    throw e;
  }
}

self.onmessage = async (e) => {
  const { id, op, sql, params, items, name } = e.data;
  try {
    let result;
    if (op === 'init') { await open(name); result = true; }
    else if (op === 'exec') { await sqlite3.exec(db, sql); result = true; }
    else if (op === 'run') result = await run(sql, params);
    else if (op === 'all') result = await all(sql, params);
    else if (op === 'get') { const r = await all(sql, params); result = r[0] || null; }
    else if (op === 'batch') { await batch(items); result = true; }
    else throw new Error('unknown op ' + op);
    self.postMessage({ id, result });
  } catch (err) {
    self.postMessage({ id, error: String(err?.message || err) });
  }
};
