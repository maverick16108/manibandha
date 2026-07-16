// Vue-реактивная витрина мессенджера над (framework-agnostic) движком.
// UI читает chatState; движок пишет в локальную SQLite и через подписку будит перезапрос.
import { reactive } from 'vue';
import { openDatabase } from './db/adapter.js';
import { chatApi } from './sync/api.js';
import { ChatEngine } from './sync/engine.js';
import { ChatSocket } from './sync/ws.js';

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
      onMessage: (evt) => engine.handleWs(evt),
      onReconnect: () => { engine.catchUp(); engine.flushOutbox(); },
      onStatus: (s) => { chatState.connection = s; },
    });
    unsub = db.subscribe(({ tables }) => onDbChange(tables));
    await engine.bootstrap();
    socket.connect();
    await refreshChats();
    chatState.ready = true;
    // страховочный догон (на случай пропущенного WS-бродкаста)
    safetyTimer = setInterval(() => { engine?.catchUp(); engine?.flushOutbox(); }, 25000);
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

export function teardownChat() {
  try { socket?.close(); } catch { /* ignore */ }
  try { unsub?.(); } catch { /* ignore */ }
  try { db?.close(); } catch { /* ignore */ }
  if (safetyTimer) clearInterval(safetyTimer);
  if (typingTimer) clearInterval(typingTimer);
  db = engine = socket = unsub = safetyTimer = typingTimer = null;
  chatState.ready = false;
  chatState.chats = [];
  chatState.messages = [];
  chatState.activeChatId = null;
  chatState.totalUnread = 0;
}

async function onDbChange(tables) {
  const t = tables || [];
  if (t.includes('chats') || t.includes('members')) await refreshChats();
  if (t.includes('messages')) {
    if (chatState.activeChatId) await refreshMessages();
    await refreshChats(); // превью/порядок в списке
  }
}

async function refreshChats() {
  if (!db) return;
  const chats = await db.all('SELECT * FROM chats ORDER BY (updated_at IS NULL), updated_at DESC');
  const mem = await db.all('SELECT * FROM members');
  const out = [];
  let total = 0;
  for (const c of chats) {
    const members = mem.filter((m) => m.chat_id === c.id);
    const peer = c.type === 'direct' ? members.find((m) => m.user_id !== chatState.meId) : null;
    const last = await db.get(
      'SELECT body,author_id,created_at,seq,deleted,status FROM messages WHERE chat_id=? ORDER BY (seq IS NULL), seq DESC, local_ts DESC LIMIT 1',
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
      updated_at: c.updated_at,
      last,
    });
  }
  chatState.chats = out;
  chatState.totalUnread = total;
}

async function refreshMessages() {
  if (!db || !chatState.activeChatId) return;
  chatState.messages = await db.all(
    'SELECT * FROM messages WHERE chat_id=? ORDER BY (seq IS NULL), seq ASC, local_ts ASC',
    [chatState.activeChatId],
  );
  chatState.members = await db.all('SELECT * FROM members WHERE chat_id=?', [chatState.activeChatId]);
}

export async function openChat(chatId) {
  chatState.activeChatId = Number(chatId);
  await refreshMessages();
  await engine?.ensureChatMessages(chatState.activeChatId);
  await markReadNow();
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

export async function sendMessage(body, replyToId = null) {
  if (!chatState.activeChatId) return;
  await engine?.send(chatState.activeChatId, body, replyToId);
  await markReadNow();
}

export function sendTyping() {
  if (chatState.activeChatId) socket?.sendTyping(chatState.activeChatId);
}

export async function editMessage(messageId, body) {
  await engine?.editMessage(chatState.activeChatId, messageId, body);
}

export async function deleteMessage(messageId) {
  await engine?.deleteMessage(chatState.activeChatId, messageId);
}

export async function retryFailed() {
  await engine?.retryFailed();
}

export async function loadOlder() {
  if (!chatState.activeChatId || !chatState.messages.length) return 0;
  const oldest = chatState.messages.find((m) => m.seq != null);
  if (!oldest) return 0;
  const n = await engine?.ensureChatMessages(chatState.activeChatId, oldest.seq);
  await refreshMessages();
  return n;
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
