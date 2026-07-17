// Framework-agnostic витрина мессенджера: держит состояние, подписки и действия.
// Web оборачивает в reactive-мост, mobile — в useSyncExternalStore(subscribe, getSnapshot).
import type { DbAdapter } from './db/adapter.js';
import type { ChatApi } from './sync/api.js';
import type { ChatSocket } from './sync/socket.js';
import { ChatEngine } from './sync/engine.js';
import type { Chat, ChatMember, ChatMessage, Contact } from './types.js';

export interface ChatListItem {
  id: number;
  type: 'direct' | 'group';
  title: string;
  avatar_url: string | null;
  members: ChatMember[];
  unread: number;
  pinned: boolean;
  updated_at: string | null;
  last: (Partial<ChatMessage> & { deleted?: boolean }) | null;
}

export interface ChatState {
  ready: boolean;
  connection: 'online' | 'offline';
  meId: number | null;
  chats: ChatListItem[];
  totalUnread: number;
  activeChatId: number | null;
  unreadBeforeSeq: number;
  messages: ChatMessage[];
  members: ChatMember[];
  typing: Record<number, { name?: string; ts: number }>;
  contacts: Contact[];
}

export interface ChatStoreDeps {
  db: DbAdapter;
  api: ChatApi;
  meId: number;
  /** Фабрика сокета — платформа передаёт готовую реализацию (createReconnectingSocket). */
  makeSocket(handlers: {
    onMessage(evt: unknown): void;
    onReconnect(): void;
    onStatus(s: 'online' | 'offline'): void;
  }): ChatSocket;
  genUuid?: () => string;
  now?: () => number;
}

const initialState = (): ChatState => ({
  ready: false,
  connection: 'offline',
  meId: null,
  chats: [],
  totalUnread: 0,
  activeChatId: null,
  unreadBeforeSeq: 0,
  messages: [],
  members: [],
  typing: {},
  contacts: [],
});

export class ChatStore {
  private db: DbAdapter;
  private api: ChatApi;
  private engine: ChatEngine;
  private socket: ChatSocket;
  private unsub: (() => void) | null = null;
  private listeners = new Set<() => void>();
  private safetyTimer: ReturnType<typeof setInterval> | null = null;
  private typingTimer: ReturnType<typeof setInterval> | null = null;
  private state: ChatState = initialState();

  constructor(private deps: ChatStoreDeps) {
    this.db = deps.db;
    this.api = deps.api;
    this.state.meId = deps.meId;
    this.engine = new ChatEngine({
      db: deps.db,
      api: deps.api,
      meId: deps.meId,
      genUuid: deps.genUuid,
      now: deps.now,
      onEphemeral: (e) => {
        if (e.type === 'typing') {
          this.patch({ typing: { ...this.state.typing, [e.chatId]: { name: e.name, ts: Date.now() } } });
        }
      },
    });
    this.socket = deps.makeSocket({
      onMessage: (evt) => void this.engine.handleWs(evt),
      onReconnect: () => { void this.engine.catchUp(); void this.engine.flushOutbox(); },
      onStatus: (s) => this.patch({ connection: s }),
    });
  }

  // ── подписка / снапшот (для useSyncExternalStore) ──────────────────────
  subscribe = (cb: () => void): (() => void) => {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  };
  getSnapshot = (): ChatState => this.state;

  private patch(part: Partial<ChatState>): void {
    this.state = { ...this.state, ...part };
    for (const cb of this.listeners) cb();
  }

  // ── жизненный цикл ──────────────────────────────────────────────────────
  async init(): Promise<void> {
    this.unsub = this.db.subscribe(({ tables }) => void this.onDbChange(tables));
    try { await this.engine.bootstrap(); } catch (e) { console.warn('[chat] bootstrap failed', e); }
    this.socket.connect();
    try { await this.refreshChats(); } catch (e) { console.warn('[chat] refreshChats failed', e); }
    this.patch({ ready: true });
    this.safetyTimer = setInterval(() => { void this.engine.catchUp(); void this.engine.flushOutbox(); }, 25000);
    this.typingTimer = setInterval(() => {
      const now = Date.now();
      const next = { ...this.state.typing };
      let changed = false;
      for (const k of Object.keys(next)) {
        const id = Number(k);
        if (now - next[id].ts > 4000) { delete next[id]; changed = true; }
      }
      if (changed) this.patch({ typing: next });
    }, 1500);
  }

  async teardown(): Promise<void> {
    try { this.socket.close(); } catch { /* ignore */ }
    try { this.unsub?.(); } catch { /* ignore */ }
    try { await this.db.close(); } catch { /* ignore */ }
    if (this.safetyTimer) clearInterval(this.safetyTimer);
    if (this.typingTimer) clearInterval(this.typingTimer);
    this.state = initialState();
    for (const cb of this.listeners) cb();
  }

  private async onDbChange(tables: string[]): Promise<void> {
    const t = tables || [];
    if (t.includes('chats') || t.includes('members')) await this.refreshChats();
    if (t.includes('messages')) {
      if (this.state.activeChatId) await this.refreshMessages();
      await this.refreshChats();
    }
  }

  private async refreshChats(): Promise<void> {
    const chats = await this.db.all<Record<string, any>>('SELECT * FROM chats ORDER BY pinned DESC, (updated_at IS NULL), updated_at DESC');
    const mem = await this.db.all<ChatMember>('SELECT * FROM members');
    const out: ChatListItem[] = [];
    let total = 0;
    for (const c of chats) {
      const members = mem.filter((m) => m.chat_id === c.id);
      const peer = c.type === 'direct' ? members.find((m) => m.user_id !== this.state.meId) : null;
      const last = await this.db.get<any>(
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
        updated_at: c.updated_at,
        last,
      });
    }
    this.patch({ chats: out, totalUnread: total });
  }

  private async refreshMessages(): Promise<void> {
    if (!this.state.activeChatId) return;
    const messages = await this.db.all<ChatMessage>(
      'SELECT * FROM messages WHERE chat_id=? AND deleted=0 AND (hidden IS NULL OR hidden=0) ORDER BY (seq IS NULL), seq ASC, local_ts ASC',
      [this.state.activeChatId],
    );
    const members = await this.db.all<ChatMember>('SELECT * FROM members WHERE chat_id=?', [this.state.activeChatId]);
    this.patch({ messages, members });
  }

  private async markReadNow(): Promise<void> {
    let maxSeq = 0;
    for (const m of this.state.messages) if (m.seq && m.seq > maxSeq) maxSeq = m.seq;
    if (maxSeq && this.state.activeChatId) await this.engine.markRead(this.state.activeChatId, maxSeq);
  }

  // ── действия ────────────────────────────────────────────────────────────
  async openChat(chatId: number): Promise<void> {
    const id = Number(chatId);
    this.patch({ activeChatId: id });
    try {
      const row = await this.db.get<{ my_last_read_seq: number }>('SELECT my_last_read_seq FROM chats WHERE id=?', [id]);
      this.patch({ unreadBeforeSeq: row ? (row.my_last_read_seq || 0) : 0 });
    } catch { this.patch({ unreadBeforeSeq: 0 }); }
    await this.refreshMessages();
    await this.engine.ensureChatMessages(id);
    await this.markReadNow();
  }
  closeChat(): void { this.patch({ activeChatId: null, messages: [], members: [] }); }

  async sendMessage(body: string, replyToId: number | null = null, replyQuote: string | null = null): Promise<void> {
    if (!this.state.activeChatId) return;
    await this.engine.send(this.state.activeChatId, body, replyToId, replyQuote);
    await this.markReadNow();
  }
  sendTyping(): void { if (this.state.activeChatId) this.socket.sendTyping(this.state.activeChatId); }
  async editMessage(messageId: number, body: string): Promise<void> { if (this.state.activeChatId) await this.engine.editMessage(this.state.activeChatId, messageId, body); }
  async deleteMessage(messageId: number, forEveryone: boolean): Promise<void> {
    if (!this.state.activeChatId) return;
    if (forEveryone) await this.engine.deleteMessage(this.state.activeChatId, messageId);
    else await this.engine.hideMessage(this.state.activeChatId, messageId);
  }
  async reactMessage(messageId: number, emoji: string): Promise<void> { if (this.state.activeChatId) await this.engine.react(this.state.activeChatId, messageId, emoji); }
  async retryFailed(): Promise<void> { await this.engine.retryFailed(); }
  async loadOlder(): Promise<number> {
    if (!this.state.activeChatId || !this.state.messages.length) return 0;
    const oldest = this.state.messages.find((m) => m.seq != null);
    if (!oldest) return 0;
    const n = await this.engine.ensureChatMessages(this.state.activeChatId, oldest.seq);
    await this.refreshMessages();
    return n;
  }
  async loadContacts(): Promise<void> { try { this.patch({ contacts: await this.api.contacts() }); } catch { this.patch({ contacts: [] }); } }
  async startDirect(peerId: number): Promise<number> { const id = await this.engine.createChat({ type: 'direct', peer_id: peerId }); await this.refreshChats(); return id; }
  async startGroup(title: string, memberIds: number[]): Promise<number> { const id = await this.engine.createChat({ type: 'group', title, member_ids: memberIds }); await this.refreshChats(); return id; }
  async updateChat(chatId: number, payload: { title?: string; photo_url?: string | null }): Promise<void> { await this.engine.updateChat(chatId, payload); }
  async pinChat(chatId: number, pinned: boolean): Promise<void> { await this.engine.pinChat(chatId, pinned); }
  async leaveChat(chatId: number): Promise<void> { await this.engine.leaveChat(chatId); if (this.state.activeChatId === Number(chatId)) this.closeChat(); await this.refreshChats(); }
}
