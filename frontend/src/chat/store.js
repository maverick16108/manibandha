// Vue-реактивная витрина мессенджера над (framework-agnostic) движком.
// UI читает chatState; движок пишет в локальную SQLite и через подписку будит перезапрос.
import { reactive } from 'vue';
import { openDatabase } from './db/adapter.js';
import { chatApi } from './sync/api.js';
import { ChatEngine } from './sync/engine.js';
import { ChatSocket } from './sync/ws.js';
import { thumbUrl } from '../lib/format';

let db = null;
let engine = null;
let socket = null;
let unsub = null;
let safetyTimer = null;
let typingTimer = null;
let starting = null;

export const chatState = reactive({
  ready: false,
  connection: 'offline',   // online | offline
  meId: null,
  chats: [],               // список для сайдбара (с вычисленным заголовком/аватаром/превью)
  totalUnread: 0,
  activeChatId: null,
  unreadBeforeSeq: 0,      // граница непрочитанного на момент открытия чата
  messages: [],            // сообщения активного чата
  members: [],             // участники активного чата
  typing: {},              // chatId -> { name, ts }
  contacts: [],
});

function onEphemeral(e) {
  if (e.type === 'typing') {
    chatState.typing = { ...chatState.typing, [e.chatId]: { name: e.name, ts: Date.now() } };
  }
}

export async function initChat({ meId, getToken }) {
  if (engine) return;
  if (starting) return starting;
  starting = (async () => {
    chatState.meId = meId;
    db = await openDatabase();
    engine = new ChatEngine({ db, api: chatApi, meId, onEphemeral });
    socket = new ChatSocket({
      getToken,
      onMessage: (evt) => { if (evt && evt.type === 'call') { callHandler && callHandler(evt); } else { engine.handleWs(evt); } },
      onReconnect: () => {
        engine.catchUp(); engine.flushOutbox();
        // пока были офлайн, могли пропустить delete/edit (они не двигают seq → догон их не берёт).
        // Пересинхроним открытый чат: listMessages вернёт актуальные флаги deleted/edited.
        if (chatState.activeChatId) engine.ensureChatMessages(chatState.activeChatId);
      },
      onStatus: (s) => { chatState.connection = s; },
    });
    unsub = db.subscribe(({ tables }) => onDbChange(tables));
    try { await engine.bootstrap(); } catch (e) { console.warn('[chat] bootstrap failed', e); }
    socket.connect();
    try { await refreshChats(); } catch (e) { console.warn('[chat] refreshChats failed', e); }
    prefetchPhotos(); // прогреть миниатюры фото в кэш — мгновенный показ при открытии чата
    chatState.ready = true; // показываем список даже если что-то пошло не так
    // страховочная сверка: догон по глобальному pts + авторитетная сверка ХВОСТА
    // открытого чата (закрывает любые расхождения, даже без разрыва связи) + дошлём outbox
    safetyTimer = setInterval(() => { resync(); }, 15000);
    // немедленная сверка при возврате в сеть / на вкладку
    window.addEventListener('online', resync);
    document.addEventListener('visibilitychange', onVisibleResync);
    window.addEventListener('focus', resync);
    // очистка индикатора «печатает…»
    typingTimer = setInterval(() => {
      const now = Date.now();
      let changed = false;
      const next = { ...chatState.typing };
      for (const k of Object.keys(next)) if (now - next[k].ts > 4000) { delete next[k]; changed = true; }
      if (changed) chatState.typing = next;
    }, 1500);
  })();
  try { await starting; } finally { starting = null; }
}

// Полная сверка с сервером: догон по pts, дошли outbox, авторитетно сверить хвост
// открытого чата. Идемпотентно и без лишних перерисовок (см. engine.ensureChatMessages).
function resync() {
  if (!engine) return;
  engine.catchUp();
  engine.flushOutbox();
  if (chatState.activeChatId) engine.ensureChatMessages(chatState.activeChatId);
  prefetchPhotos();
}
function onVisibleResync() { if (document.visibilityState === 'visible') resync(); }

// Прогрев миниатюр в кэш браузера: тянем фото из недавних сообщений всех чатов ЗАРАНЕЕ,
// чтобы при открытии чата картинки показывались мгновенно (не «через секунду»).
const warmed = new Set();
// запоминаем пропорции (w/h) миниатюр, чтобы зарезервировать место и лента не «прыгала»
const IMG_DIMS_KEY = 'chatImgDims';
let imgDims = {}; try { imgDims = JSON.parse(localStorage.getItem(IMG_DIMS_KEY) || '{}') || {}; } catch { imgDims = {}; }
let dimsTimer = null;
function saveDims() { if (dimsTimer) return; dimsTimer = setTimeout(() => { dimsTimer = null; try { localStorage.setItem(IMG_DIMS_KEY, JSON.stringify(imgDims)); } catch { /* ignore */ } }, 800); }
export function imageAspect(url) { const t = thumbUrl(url); return imgDims[t] || imgDims[url] || null; }
function warmImage(url) {
  if (!url || warmed.has(url)) return; warmed.add(url);
  try {
    const i = new Image(); i.decoding = 'async';
    i.onload = () => { if (i.naturalWidth && i.naturalHeight) { imgDims[url] = Math.round((i.naturalWidth / i.naturalHeight) * 1000) / 1000; saveDims(); } };
    i.src = url;
  } catch { /* ignore */ }
}
async function prefetchPhotos() {
  if (!db) return;
  try {
    const rows = await db.all(
      "SELECT body FROM messages WHERE deleted=0 AND (body LIKE '%![]%' OR body LIKE '%@[video]%') ORDER BY (seq IS NULL), seq DESC LIMIT 400",
    );
    for (const r of rows) {
      const b = r.body || '';
      b.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (_x, u) => { warmImage(thumbUrl(u)); return ''; });
      // постеры видео тоже прогреваем (для мгновенного показа и точного резерва места)
      b.replace(/@\[video\]\([^|)]+\|([^|)]*)/g, (_x, poster) => { if (poster) warmImage(thumbUrl(poster)); return ''; });
    }
  } catch { /* ignore */ }
}

export function teardownChat() {
  try { socket?.close(); } catch { /* ignore */ }
  try { unsub?.(); } catch { /* ignore */ }
  try { db?.close(); } catch { /* ignore */ }
  try { window.removeEventListener('online', resync); } catch { /* ignore */ }
  try { document.removeEventListener('visibilitychange', onVisibleResync); } catch { /* ignore */ }
  try { window.removeEventListener('focus', resync); } catch { /* ignore */ }
  if (safetyTimer) clearInterval(safetyTimer);
  if (typingTimer) clearInterval(typingTimer);
  if (refreshChatsTimer) { clearTimeout(refreshChatsTimer); refreshChatsTimer = null; }
  db = engine = socket = unsub = safetyTimer = typingTimer = null;
  chatState.ready = false;
  chatState.chats = [];
  chatState.messages = [];
  chatState.activeChatId = null;
  chatState.totalUnread = 0;
}

// refreshChats дорогой (N+1 запрос «последнее сообщение» по каждому чату) — при
// быстрой отправке он дёргается на каждое сообщение и даёт «залипания». Коалесцируем:
// активный чат обновляем мгновенно (refreshMessages), а список — раз в ~120мс.
let refreshChatsTimer = null;
function scheduleRefreshChats() {
  if (refreshChatsTimer) clearTimeout(refreshChatsTimer);
  refreshChatsTimer = setTimeout(() => { refreshChatsTimer = null; refreshChats(); }, 120);
}

async function onDbChange(tables) {
  const t = tables || [];
  if (t.includes('chats') || t.includes('members')) scheduleRefreshChats();
  // событие «прочитано» меняет только members → обновляем участников активного чата,
  // иначе peerReadSeq остаётся устаревшим и вторая галочка (прочитано) не появляется
  if (t.includes('members') && chatState.activeChatId) {
    chatState.members = await db.all('SELECT * FROM members WHERE chat_id=?', [chatState.activeChatId]);
  }
  if (t.includes('messages')) {
    if (chatState.activeChatId) await refreshMessages();
    scheduleRefreshChats(); // превью/порядок в списке — коалесцированно
  }
}

async function refreshChats() {
  if (!db) return;
  const chats = await db.all('SELECT * FROM chats ORDER BY pinned DESC, pin_order ASC, (updated_at IS NULL), updated_at DESC');
  const mem = await db.all('SELECT * FROM members');
  const out = [];
  let total = 0;
  for (const c of chats) {
    const members = mem.filter((m) => m.chat_id === c.id);
    const peer = c.type === 'direct' ? members.find((m) => m.user_id !== chatState.meId) : null;
    const last = await db.get(
      'SELECT body,author_id,author_name,created_at,seq,deleted,status FROM messages WHERE chat_id=? AND deleted=0 AND (hidden IS NULL OR hidden=0) ORDER BY (seq IS NULL), seq DESC, local_ts DESC LIMIT 1',
      [c.id],
    );
    total += c.unread || 0;
    out.push({
      id: c.id,
      type: c.type,
      title: c.type === 'group' ? (c.title || 'Группа') : (peer?.full_name || 'Диалог'),
      avatar_url: c.type === 'group' ? c.photo_url : (peer?.avatar_url || null),
      members,
      unread: c.unread || 0,
      pinned: !!c.pinned,
      pinned_message_id: c.pinned_message_id || null,
      updated_at: c.updated_at,
      last,
    });
  }
  chatState.chats = out;
  chatState.totalUnread = total;
}

// Рендерим ОКНО последних сообщений (не всю историю) — иначе на больших чатах отрисовка
// тысяч нод тормозит открытие. Прокрутка вверх расширяет окно (loadOlder).
const MSG_WINDOW = 200;
let msgWindow = MSG_WINDOW;
async function refreshMessages() {
  if (!db || !chatState.activeChatId) return;
  const rows = await db.all(
    `SELECT * FROM (
       SELECT * FROM messages WHERE chat_id=? AND deleted=0 AND (hidden IS NULL OR hidden=0)
       ORDER BY (seq IS NULL) DESC, seq DESC, local_ts DESC LIMIT ?
     ) t ORDER BY (seq IS NULL), seq ASC, local_ts ASC`,
    [chatState.activeChatId, msgWindow],
  );
  chatState.messages = rows;
  chatState.members = await db.all('SELECT * FROM members WHERE chat_id=?', [chatState.activeChatId]);
}

const chatWindowMem = {}; // chatId → msgWindow: сохраняем размер окна рендера, чтобы при
                          // возврате контент был той же высоты (иначе сохранённый scrollTop «съезжает»)
export async function openChat(chatId) {
  const id = Number(chatId);
  if (chatState.activeChatId && chatState.activeChatId !== id) chatWindowMem[chatState.activeChatId] = msgWindow;
  chatState.activeChatId = id;
  msgWindow = chatWindowMem[id] || MSG_WINDOW;
  // граница непрочитанного до показа — из локальной БД (быстро), для разделителя «Непрочитанные»
  try {
    const row = await db.get('SELECT my_last_read_seq FROM chats WHERE id=?', [id]);
    chatState.unreadBeforeSeq = row ? (row.my_last_read_seq || 0) : 0;
  } catch { chatState.unreadBeforeSeq = 0; }
  // АТОМАРНО подменяем содержимое на новый чат (без пустого мигания): читаем и присваиваем разом
  await refreshMessages();
  const c = chatState.chats.find((x) => x.id === id);
  if (c) c.unread = 0;
  prefetchPhotos();          // прогрев миниатюр открытого чата
  markReadNow();
  // авторитетная сверка с сервером — в ФОНЕ, не блокируем показ (перерисует, только если изменилось)
  engine?.ensureChatMessages(id).then(() => { if (chatState.activeChatId === id) markReadNow(); });
}

export async function pinChat(chatId, pinned) {
  await engine?.pinChat(chatId, pinned);
}

// закрепить/открепить сообщение в чате (оптимистично локально + на сервер)
export async function pinMessageInChat(chatId, messageId) {
  if (!db) return;
  await db.run('UPDATE chats SET pinned_message_id=? WHERE id=?', [messageId, chatId], ['chats']);
  try { await chatApi.pinMessage(chatId, messageId); } catch { /* сверится позже */ }
}
export async function unpinMessageInChat(chatId) {
  if (!db) return;
  await db.run('UPDATE chats SET pinned_message_id=NULL WHERE id=?', [chatId], ['chats']);
  try { await chatApi.unpinMessage(chatId); } catch { /* сверится позже */ }
}

// новый порядок закреплённых (перетаскивание): оптимистично локально + на сервер
export async function reorderPins(ids) {
  if (!db || !ids?.length) return;
  for (let i = 0; i < ids.length; i++) {
    await db.run('UPDATE chats SET pin_order=? WHERE id=?', [i, ids[i]], []);
  }
  await refreshChats();
  try { await chatApi.reorderPins(ids); } catch { /* сверится при следующем listChats */ }
}

export async function leaveChat(chatId) {
  await engine?.leaveChat(chatId);
  if (chatState.activeChatId === Number(chatId)) closeChat();
  await refreshChats();
}

export function closeChat() {
  chatState.activeChatId = null;
  chatState.messages = [];
  chatState.members = [];
}

async function markReadNow() {
  const msgs = chatState.messages;
  let maxSeq = 0;
  for (const m of msgs) if (m.seq && m.seq > maxSeq) maxSeq = m.seq;
  if (maxSeq) await engine?.markRead(chatState.activeChatId, maxSeq);
}

// отметить активный чат прочитанным (для авто-чтения живых сообщений при просмотре)
export async function markActiveRead() { await markReadNow(); }

export async function sendMessage(body, replyToId = null, replyQuote = null) {
  if (!chatState.activeChatId) return;
  await engine?.send(chatState.activeChatId, body, replyToId, replyQuote);
  await markReadNow();
}

// Отправка в КОНКРЕТНЫЙ чат (для фоновых загрузок: активный чат мог смениться,
// пока грузилось видео/файл — сообщение должно уйти в свой чат, а не в текущий).
export async function sendMessageTo(chatId, body) {
  if (!chatId || !(body || '').trim()) return;
  await engine?.send(chatId, body);
}

export async function updateChat(chatId, payload) {
  await engine?.updateChat(chatId, payload);
}

// Статистика локального кэша (сообщения/чаты в браузерной БД) — для «Памяти устройства».
export async function localCacheStats() {
  const out = { messages: 0, chats: 0 };
  try {
    const r1 = await engine?.db.all('SELECT COUNT(*) AS n FROM messages');
    const r2 = await engine?.db.all('SELECT COUNT(*) AS n FROM chats');
    out.messages = Number(r1?.[0]?.n) || 0;
    out.chats = Number(r2?.[0]?.n) || 0;
  } catch { /* БД ещё не готова */ }
  return out;
}

// Полная очистка локального кэша чатов: закрываем БД, сносим IndexedDB/OPFS и chat-ключи
// localStorage. Данные не теряются — при следующем открытии всё подтянется с сервера.
export async function wipeLocalChatCache() {
  try { teardownChat(); } catch { /* ignore */ }
  // IndexedDB (снапшот sql.js-фолбэка и пр.)
  try {
    if (indexedDB.databases) {
      const dbs = await indexedDB.databases();
      await Promise.all((dbs || []).map((d) => d && d.name && new Promise((res) => {
        const req = indexedDB.deleteDatabase(d.name); req.onsuccess = req.onerror = req.onblocked = () => res();
      })));
    } else {
      for (const n of ['manibandha-chat-sqljs']) await new Promise((res) => { const r = indexedDB.deleteDatabase(n); r.onsuccess = r.onerror = r.onblocked = () => res(); });
    }
  } catch { /* ignore */ }
  // OPFS (wa-sqlite)
  try {
    const root = await navigator.storage?.getDirectory?.();
    if (root) { for await (const [name] of root.entries()) { try { await root.removeEntry(name, { recursive: true }); } catch { /* занят */ } } }
  } catch { /* ignore */ }
  // локальные кэши превью/метаданных
  try { for (const k of Object.keys(localStorage)) if (/^chat/i.test(k)) localStorage.removeItem(k); } catch { /* ignore */ }
}

// Переслать сообщения (их тело) в другой чат — по очереди, с сохранением порядка.
export async function forwardMessages(targetChatId, bodies) {
  if (!targetChatId || !engine) return;
  for (const b of bodies) {
    if (b && b.trim()) await engine.send(targetChatId, b);
  }
}

export function sendTyping() {
  if (chatState.activeChatId) socket?.sendTyping(chatState.activeChatId);
}

// ── сигналинг звонков (WebRTC поверх чат-сокета) ────────────────────────────
let callHandler = null;
export function onCallSignal(fn) { callHandler = fn; }
export function sendCallSignal(obj) { socket?.send({ type: 'call', ...obj }); }

export async function editMessage(messageId, body) {
  await engine?.editMessage(chatState.activeChatId, messageId, body);
}

export async function deleteMessage(messageId, forEveryone) {
  if (forEveryone) await engine?.deleteMessage(chatState.activeChatId, messageId);
  else await engine?.hideMessage(chatState.activeChatId, messageId);
}

export const REACTION_EMOJIS = ['❤️', '👍', '🙏', '🔥', '😂', '🎉'];

export async function reactMessage(messageId, emoji) {
  if (!messageId) return;
  await engine?.react(chatState.activeChatId, messageId, emoji);
}

export async function retryFailed() {
  await engine?.retryFailed();
}

export async function loadOlder() {
  if (!chatState.activeChatId || !chatState.messages.length) return 0;
  const oldest = chatState.messages.find((m) => m.seq != null);
  if (!oldest) return 0;
  // расширяем окно локального рендера и, при нужде, подтягиваем историю с сервера
  msgWindow += MSG_WINDOW;
  const n = await engine?.ensureChatMessages(chatState.activeChatId, oldest.seq);
  await refreshMessages();
  return n;
}

// расширить окно локального рендера (для перехода к процитированному сообщению вверху истории)
export async function expandWindow() {
  msgWindow += MSG_WINDOW;
  await refreshMessages();
  return chatState.messages.length;
}

// подгрузить окно сообщений вокруг seq (для перехода к результату поиска)
export async function loadAroundSeq(seq) {
  if (!chatState.activeChatId || !seq) return
  await engine?.ensureChatMessages(chatState.activeChatId, Number(seq) + 1);
  await refreshMessages();
}

export async function loadContacts() {
  try { chatState.contacts = await chatApi.contacts(); } catch { chatState.contacts = []; }
}

export async function startDirect(peerId) {
  const id = await engine.createChat({ type: 'direct', peer_id: peerId });
  await refreshChats();
  return id;
}

export async function startGroup(title, memberIds) {
  const id = await engine.createChat({ type: 'group', title, member_ids: memberIds });
  await refreshChats();
  return id;
}

export function chatReady() { return !!engine; }
