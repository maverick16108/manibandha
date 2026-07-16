// Единый интерфейс локальной БД. За ним — реализация под платформу:
//   web:    wa-sqlite + OPFS (персистентно, в Worker) с фолбэком на sql.js (in-memory)
//   RN:     op-sqlite (позже)   ·   Flutter: sqlite3 (позже)
//
// Контракт адаптера:
//   await init()                                  — открыть БД, применить схему
//   await exec(sql)                               — выполнить DDL/несколько операторов без параметров
//   await run(sql, params=[], tables=[])          — один оператор записи → { changes }; эмитит change
//   await all(sql, params=[])                     — массив строк-объектов
//   await get(sql, params=[])                     — первая строка или null
//   await batch(items=[{sql,params}], tables=[])  — атомарно (BEGIN/COMMIT); эмитит change
//   subscribe(cb) -> unsubscribe                  — cb({ tables }) после каждой записи
//   await close()
//
// Реактивность: после любой записи адаптер зовёт подписчиков с затронутыми таблицами,
// UI-стор перезапрашивает нужные представления и перерисовывает списки.

export function supportsOpfsWorker() {
  // Дешёвый гейт. Метод createSyncAccessHandle доступен только ВНУТРИ воркера,
  // поэтому на главном потоке его не проверяем — реальную способность проверит
  // инициализация воркера (при неудаче openDatabase упадёт в фолбэк sql.js).
  try {
    return (
      typeof Worker !== 'undefined' &&
      typeof navigator !== 'undefined' &&
      !!navigator.storage?.getDirectory &&
      typeof FileSystemFileHandle !== 'undefined'
    );
  } catch {
    return false;
  }
}

// Открыть локальную БД, выбрав лучшую доступную реализацию.
export async function openDatabase(name = 'manibandha-chat') {
  if (supportsOpfsWorker()) {
    try {
      const { WaSqliteAdapter } = await import('./wa-sqlite-adapter.js');
      const a = new WaSqliteAdapter(name);
      await a.init();
      return a;
    } catch (e) {
      console.warn('[chat] wa-sqlite/OPFS недоступен, фолбэк на sql.js:', e);
    }
  }
  const { SqlJsAdapter } = await import('./sqljs-adapter.js');
  const a = new SqlJsAdapter(name);
  await a.init();
  return a;
}
