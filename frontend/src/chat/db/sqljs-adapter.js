// Браузерная обёртка sql.js-адаптера: подключает wasm-ассет через Vite (?url).
import initSqlJs from 'sql.js';
import wasmUrl from 'sql.js/dist/sql-wasm.wasm?url';
import { SqlJsCore } from './sqljs-core.js';

export class SqlJsAdapter extends SqlJsCore {
  constructor(name) {
    super(name, { initSqlJs, locateFile: () => wasmUrl });
  }
}
