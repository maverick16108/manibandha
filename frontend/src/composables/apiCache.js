// Общий кеш GET-ответов разделов (SWR: показываем кеш мгновенно, ревалидируем в фоне).
// Живёт в памяти модуля — переживает SPA-навигацию, обнуляется при полной перезагрузке страницы.
import client from '../api/client';

const store = new Map(); // key -> { data, ts, promise }

// умолчания TTL по типу данных (мс)
export const TTL = {
  ref: 6 * 60 * 60 * 1000, // справочники (города/области/страны/роли/права) — редко меняются
  list: 8 * 60 * 1000,     // списки разделов
};

function keyOf(url, params) {
  if (!params || !Object.keys(params).length) return `GET:${url}`;
  const s = Object.keys(params).sort().map((k) => `${k}=${params[k]}`).join('&');
  return `GET:${url}?${s}`;
}

// синхронно достать из кеша (для мгновенного рендера без скелетона); null — нет данных
export function peekCache(url, params) {
  const e = store.get(keyOf(url, params));
  return e ? e.data : null;
}
export function isFresh(url, params, ttl = TTL.list) {
  const e = store.get(keyOf(url, params));
  return !!e && (Date.now() - e.ts) < ttl;
}

// SWR-чтение: если кеш свежий (< ttl) — вернуть без сети; иначе сходить в сеть и обновить кеш.
// При ошибке сети, если есть устаревший кеш — вернуть его (не роняем экран).
export function cachedGet(url, { params, ttl = TTL.list, force = false } = {}) {
  const key = keyOf(url, params);
  const entry = store.get(key);
  const now = Date.now();
  if (!force && entry && (now - entry.ts) < ttl) return Promise.resolve(entry.data);
  if (entry?.promise) return entry.promise; // запрос уже идёт — переиспользуем
  const promise = client.get(url, { params })
    .then((r) => { store.set(key, { data: r.data, ts: Date.now(), promise: null }); return r.data; })
    .catch((e) => { const cur = store.get(key); if (cur) { cur.promise = null; return cur.data; } throw e; });
  store.set(key, { ...(entry || {}), promise });
  return promise;
}

// инвалидация (после мутаций) — по точному ключу или по подстроке урла
export function invalidate(url, params) { store.delete(keyOf(url, params)); }
export function invalidatePrefix(sub) { for (const k of [...store.keys()]) if (k.includes(sub)) store.delete(k); }
export function clearCache() { store.clear(); }

// положить готовые данные в кеш вручную (напр. после мутации сервер вернул свежий список)
export function primeCache(url, params, data) { store.set(keyOf(url, params), { data, ts: Date.now(), promise: null }); }
