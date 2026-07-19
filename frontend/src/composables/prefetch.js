// Фоновый прогрев данных разделов после входа: тихо, по одному запросу с паузой (стаггеринг),
// низким приоритетом (requestIdleCallback). Заполняет общий кеш (apiCache) — разделы затем
// читают из него и показываются без скелетонов. Управляется настройкой localStorage 'apiPrefetch'.
import { cachedGet, TTL } from './apiCache';

// [url, params|null, ttl, capNeeded|null] — capNeeded проверяется через auth.can, чтобы не дёргать
// эндпоинты без доступа (иначе 403 в логах и лишние запросы).
const TARGETS = [
  // справочники (редко меняются, большие, нужны многим)
  ['/regions', null, TTL.ref, null],
  ['/cities', null, TTL.ref, 'disciples.view_all'],
  ['/countries', null, TTL.ref, null],
  ['/roles', null, TTL.ref, 'roles.manage'],
  ['/capabilities', null, TTL.ref, 'roles.manage'],
  // дефолтные списки разделов
  ['/disciples', null, TTL.list, 'disciples.view_all'],
  ['/threads', { kind: 'question' }, TTL.list, 'questions.view_all'],
  ['/threads', { kind: 'report' }, TTL.list, 'reports.read_all'],
  ['/forum/topics', null, TTL.list, 'forum.view'],
  ['/forum/sections', null, TTL.list, 'forum.view'],
  ['/reports/summary', null, TTL.list, 'dashboard.view'],
  ['/reports/timeline', null, TTL.list, 'dashboard.view'],
  ['/events', null, TTL.list, 'calendar.view'],
  ['/conferences', null, TTL.list, 'conference.view'],
  ['/users', null, TTL.list, 'users.manage'],
];

let started = false;
export function prefetchSections(can) {
  if (started) return; started = true;
  if (typeof localStorage !== 'undefined' && localStorage.getItem('apiPrefetch') === '0') return;
  const idle = window.requestIdleCallback ? (f) => window.requestIdleCallback(f, { timeout: 3000 }) : (f) => setTimeout(f, 400);
  const queue = TARGETS
    .filter(([, , , cap]) => !cap || (typeof can === 'function' ? can(cap) : true))
    .map(([u, p, ttl]) => () => cachedGet(u, { params: p || undefined, ttl }).catch(() => {}));
  let i = 0;
  function step() {
    if (i >= queue.length) return;
    idle(() => { try { queue[i](); } catch { /* ignore */ } i += 1; setTimeout(step, 1500); }); // ~1 запрос / 1.5с
  }
  step();
}
