// Sync-движок мессенджера (framework-agnostic). Сервер — источник истины.
//
// Отправка: оптимистично пишем message(status=pending) + outbox, показываем сразу;
//   POST идемпотентен по client_uuid; после ACK — серверные id/seq, status=sent.
// Приём: WS отдаёт апдейты (применяем идемпотентным upsert по client_uuid);
//   курсор pts двигаем только догоном GET /updates?since=pts (не теряем «потерянный» бродкаст).
// Реконнект: catchUp() добирает пропущенное, flushOutbox() дошлёт неотправленное.
import type { DbAdapter } from '../db/adapter.js';
import type { ChatApi, CreateChatPayload } from './api.js';
import type { Chat, ChatMessage, EphemeralEvent } from '../types.js';

const MAX_ATTEMPTS = 5;

export interface EngineOptions {
  db: DbAdapter;
  api: ChatApi;
  meId: number;
  onEphemeral?: (e: EphemeralEvent) => void;
  genUuid?: () => string;
  now?: () => number;
}

interface OutboxRow {
  client_uuid: string;
  chat_id: number;
  body: string;
  reply_to_id: number | null;
  reply_quote: string | null;
  attempts?: number;
}

function myLastRead(chat: Chat, meId: number): number {
  const me = (chat.members || []).find((m) => m.user_id === meId);
  return me ? me.last_read_seq || 0 : 0;
}

export class ChatEngine {
  private db: DbAdapter;
  private api: ChatApi;
  readonly meId: number;
  private onEphemeral: (e: EphemeralEvent) => void;
  private genUuid: () => string;
  private now: () => number;
  private catchUpQueued = false;
  private flushing = false;
  private cuTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(opts: EngineOptions) {
    this.db = opts.db;
    this.api = opts.api;
    this.meId = opts.meId;
    this.onEphemeral = opts.onEphemeral || (() => undefined);
    this.genUuid = opts.genUuid || (() => globalThis.crypto.randomUUID());
    this.now = opts.now || (() => Date.now());
  }

  // ── sync_state / pts ──────────────────────────────────────────────────
  private async getPts(): Promise<number> {
    const row = await this.db.get<{ value: string }>("SELECT value FROM sync_state WHERE key='pts'");
    return row ? Number(row.value) || 0 : 0;
  }
  private async setPts(v: number): Promise<void> {
    await this.db.run(
      "INSERT INTO sync_state(key,value) VALUES('pts',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
      [String(v)],
    );
  }

  // ── запись чатов/сообщений в локальную БД ─────────────────────────────
  async upsertChatMeta(chat: Chat): Promise<void> {
    const items = [
      {
        sql: `INSERT INTO chats(id,type,title,photo_url,created_by,updated_at,last_seq,my_last_read_seq,unread,pinned)
              VALUES(?,?,?,?,?,?,?,?,?,?)
              ON CONFLICT(id) DO UPDATE SET type=excluded.type, title=excluded.title,
                photo_url=excluded.photo_url, created_by=excluded.created_by, updated_at=excluded.updated_at,
                pinned=excluded.pinned`,
        params: [
          chat.id, chat.type, chat.title || null, chat.photo_url || null, chat.created_by || null,
          chat.updated_at || null, chat.last_message?.seq || 0, myLastRead(chat, this.meId), chat.unread || 0,
          chat.pinned ? 1 : 0,
        ],
      },
    ];
    for (const m of chat.members || []) {
      items.push({
        sql: `INSERT INTO members(chat_id,user_id,full_name,avatar_url,role,last_read_seq)
              VALUES(?,?,?,?,?,?)
              ON CONFLICT(chat_id,user_id) DO UPDATE SET full_name=excluded.full_name,
                avatar_url=excluded.avatar_url, role=excluded.role, last_read_seq=excluded.last_read_seq`,
        params: [chat.id, m.user_id, m.full_name || null, m.avatar_url || null, m.role || 'member', m.last_read_seq || 0],
      });
    }
    await this.db.batch(items, ['chats', 'members']);
    if (chat.last_message) await this.writeMessage(chat.last_message, false);
    await this.recomputeUnread(chat.id);
  }

  private async ensureChat(chatId: number): Promise<void> {
    const c = await this.db.get('SELECT id FROM chats WHERE id=?', [chatId]);
    if (!c) {
      try {
        const chat = await this.api.getChat(chatId);
        await this.upsertChatMeta(chat);
      } catch {
        /* нет доступа/сети — сообщение всё равно сохраним */
      }
    }
  }

  private async writeMessage(m: ChatMessage, emit = true): Promise<void> {
    const uuid = m.client_uuid || `srv:${m.id}`;
    const localTs = m.created_at ? Date.parse(m.created_at) || 0 : this.now();
    await this.db.run(
      `INSERT INTO messages(chat_id,client_uuid,id,seq,author_id,author_name,body,reply_to_id,reply_preview,
                            created_at,edited_at,edit_count,deleted,reactions,my_reaction,status,local_ts)
       VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, 'sent', ?)
       ON CONFLICT(chat_id,client_uuid) DO UPDATE SET
         id=excluded.id, seq=excluded.seq, author_id=excluded.author_id, author_name=excluded.author_name,
         body=excluded.body, reply_to_id=excluded.reply_to_id, reply_preview=excluded.reply_preview,
         created_at=excluded.created_at, edited_at=excluded.edited_at, edit_count=excluded.edit_count,
         deleted=excluded.deleted, reactions=excluded.reactions, my_reaction=excluded.my_reaction, status='sent'`,
      [m.chat_id, uuid, m.id ?? null, m.seq ?? null, m.author_id ?? null, m.author_name || null,
        m.deleted ? '' : (m.body || ''), m.reply_to_id ?? null, m.reply_preview || null,
        m.created_at || null, m.edited_at || null, m.edit_count || 0, m.deleted ? 1 : 0,
        JSON.stringify(m.reactions || []), m.my_reaction || null, localTs],
      emit ? ['messages'] : [],
    );
    await this.db.run('DELETE FROM outbox WHERE client_uuid=?', [uuid], []);
    if (m.seq != null) {
      // updated_at не двигаем назад — иначе список чатов «дёргается» при подгрузке истории
      await this.db.run(
        `UPDATE chats SET last_seq=MAX(last_seq,?),
           updated_at=CASE WHEN ? > COALESCE(updated_at,'') THEN ? ELSE updated_at END WHERE id=?`,
        [m.seq, m.created_at || '', m.created_at || null, m.chat_id], [],
      );
      if (m.author_id === this.meId) {
        await this.db.run('UPDATE chats SET my_last_read_seq=MAX(my_last_read_seq,?) WHERE id=?', [m.seq, m.chat_id], []);
      }
    }
  }

  private async applyServerMessage(m: ChatMessage): Promise<void> {
    await this.ensureChat(m.chat_id);
    await this.writeMessage(m, true);
    await this.recomputeUnread(m.chat_id);
  }

  private async recomputeUnread(chatId: number): Promise<void> {
    await this.db.run(
      `UPDATE chats SET unread = (
         SELECT COUNT(*) FROM messages
         WHERE messages.chat_id = chats.id AND messages.deleted=0
           AND messages.author_id != ? AND messages.seq IS NOT NULL
           AND messages.seq > chats.my_last_read_seq
       ) WHERE id = ?`,
      [this.meId, chatId], ['chats'],
    );
  }

  // ── публичное API ──────────────────────────────────────────────────────
  async bootstrap(): Promise<void> {
    try {
      const chats = await this.api.listChats();
      for (const c of chats) await this.upsertChatMeta(c);
    } catch (e) {
      console.warn('[chat] listChats failed', e);
    }
    await this.catchUp();
    await this.flushOutbox();
  }

  async catchUp(): Promise<void> {
    if (this.catchUpQueued) return;
    this.catchUpQueued = true;
    try {
      let pts = await this.getPts();
      for (let i = 0; i < 100; i++) {
        let resp;
        try {
          resp = await this.api.getUpdates(pts);
        } catch {
          break;
        }
        for (const u of resp.updates || []) {
          if (u.type === 'message' && u.message) await this.applyServerMessage(u.message);
        }
        const np = Number(resp.pts) || pts;
        if (np <= pts) break;
        pts = np;
        await this.setPts(pts);
        if (!resp.has_more) break;
      }
    } finally {
      this.catchUpQueued = false;
    }
  }

  /** Подгрузить актуальные сообщения чата (сверка правок/удалений при открытии). */
  async ensureChatMessages(chatId: number, beforeSeq?: number | null): Promise<number> {
    try {
      const msgs = await this.api.listMessages(chatId, beforeSeq, 50);
      for (const m of msgs) await this.writeMessage(m, false);
      await this.recomputeUnread(chatId);
      await this.db.run("UPDATE sync_state SET value=value WHERE key='pts'", [], ['messages']);
      return msgs.length;
    } catch {
      return 0;
    }
  }

  async send(chatId: number, body: string, replyToId: number | null = null, replyQuote: string | null = null): Promise<string | null> {
    const text = (body || '').trim();
    if (!text) return null;
    const uuid = this.genUuid();
    const ts = this.now();
    const createdIso = new Date(ts).toISOString();
    await this.db.batch([
      {
        sql: `INSERT INTO messages(chat_id,client_uuid,id,seq,author_id,author_name,body,reply_to_id,reply_preview,
                                   created_at,edited_at,edit_count,deleted,status,local_ts)
              VALUES(?,?,NULL,NULL,?,NULL,?,?,?,?,NULL,0,0,'pending',?)`,
        params: [chatId, uuid, this.meId, text, replyToId, replyQuote || null, createdIso, ts],
      },
      {
        sql: `INSERT INTO outbox(client_uuid,chat_id,body,reply_to_id,reply_quote,created_at,attempts)
              VALUES(?,?,?,?,?,?,0)`,
        params: [uuid, chatId, text, replyToId, replyQuote || null, createdIso],
      },
      { sql: 'UPDATE chats SET updated_at=? WHERE id=?', params: [createdIso, chatId] },
    ], ['messages', 'outbox', 'chats']);
    void this.flushOutbox();
    return uuid;
  }

  async flushOutbox(): Promise<void> {
    if (this.flushing) return;
    this.flushing = true;
    try {
      const rows = await this.db.all<OutboxRow>('SELECT * FROM outbox ORDER BY created_at ASC, rowid ASC');
      for (const row of rows) await this.flushOne(row);
    } finally {
      this.flushing = false;
    }
  }

  private async flushOne(row: OutboxRow): Promise<void> {
    try {
      const m = await this.api.send(row.chat_id, {
        client_uuid: row.client_uuid, body: row.body, reply_to_id: row.reply_to_id || null, reply_quote: row.reply_quote || null,
      });
      await this.writeMessage(m, true);
      await this.recomputeUnread(row.chat_id);
    } catch {
      const attempts = (row.attempts || 0) + 1;
      const status = attempts >= MAX_ATTEMPTS ? 'failed' : 'pending';
      await this.db.run('UPDATE outbox SET attempts=? WHERE client_uuid=?', [attempts, row.client_uuid], []);
      await this.db.run('UPDATE messages SET status=? WHERE chat_id=? AND client_uuid=?',
        [status, row.chat_id, row.client_uuid], ['messages']);
    }
  }

  async retryFailed(): Promise<void> {
    await this.db.run('UPDATE outbox SET attempts=0', [], []);
    await this.db.run("UPDATE messages SET status='pending' WHERE status='failed'", [], ['messages']);
    await this.flushOutbox();
  }

  async markRead(chatId: number, seq: number): Promise<void> {
    if (!seq) return;
    await this.db.run('UPDATE chats SET my_last_read_seq=MAX(my_last_read_seq,?), unread=0 WHERE id=?', [seq, chatId], ['chats']);
    await this.db.run('UPDATE members SET last_read_seq=MAX(last_read_seq,?) WHERE chat_id=? AND user_id=?',
      [seq, chatId, this.meId], ['members']);
    try {
      await this.api.markRead(chatId, seq);
    } catch {
      /* дошлём позже */
    }
  }

  async editMessage(chatId: number, messageId: number, body: string): Promise<void> {
    const m = await this.api.editMessage(chatId, messageId, body);
    await this.writeMessage(m, true);
  }

  async deleteMessage(chatId: number, messageId: number): Promise<void> {
    await this.api.deleteMessage(chatId, messageId);
    await this.db.run("UPDATE messages SET deleted=1, body='' WHERE chat_id=? AND id=?", [chatId, messageId], ['messages']);
  }

  /** «Удалить для себя» — скрываем локально, на сервер не ходим. */
  async hideMessage(chatId: number, messageId: number): Promise<void> {
    await this.db.run('UPDATE messages SET hidden=1 WHERE chat_id=? AND id=?', [chatId, messageId], ['messages']);
  }

  async react(chatId: number, messageId: number, emoji: string): Promise<void> {
    const res = await this.api.react(chatId, messageId, emoji);
    await this.db.run('UPDATE messages SET reactions=?, my_reaction=? WHERE chat_id=? AND id=?',
      [JSON.stringify(res.reactions || []), res.my_reaction || null, chatId, messageId], ['messages']);
  }

  async createChat(payload: CreateChatPayload): Promise<number> {
    const c = await this.api.createChat(payload);
    await this.upsertChatMeta(c);
    return c.id;
  }

  async updateChat(chatId: number, payload: { title?: string; photo_url?: string | null }): Promise<void> {
    const c = await this.api.updateChat(chatId, payload);
    await this.upsertChatMeta(c);
  }

  async pinChat(chatId: number, pinned: boolean): Promise<void> {
    await this.api.pin(chatId, pinned);
    await this.db.run('UPDATE chats SET pinned=? WHERE id=?', [pinned ? 1 : 0, chatId], ['chats']);
  }

  async leaveChat(chatId: number): Promise<void> {
    await this.api.leaveChat(chatId);
    await this.db.batch([
      { sql: 'DELETE FROM messages WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM members WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM outbox WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM chats WHERE id=?', params: [chatId] },
    ], ['chats', 'messages']);
  }

  // ── обработка WS-апдейтов ───────────────────────────────────────────────
  async handleWs(evt: any): Promise<void> {
    switch (evt?.type) {
      case 'message':
        if (evt.message) {
          await this.applyServerMessage(evt.message);
          this.queueCatchUp();
        }
        break;
      case 'edit':
        if (evt.message) await this.writeMessage(evt.message, true);
        break;
      case 'delete':
        await this.db.run("UPDATE messages SET deleted=1, body='' WHERE chat_id=? AND id=?",
          [evt.chat_id, evt.message_id], ['messages']);
        break;
      case 'read':
        await this.db.run('UPDATE members SET last_read_seq=MAX(last_read_seq,?) WHERE chat_id=? AND user_id=?',
          [evt.last_read_seq, evt.chat_id, evt.user_id], ['members']);
        break;
      case 'react': {
        await this.db.run('UPDATE messages SET reactions=? WHERE chat_id=? AND id=?',
          [JSON.stringify(evt.reactions || []), evt.chat_id, evt.message_id], ['messages']);
        if (evt.user_id === this.meId) {
          await this.db.run('UPDATE messages SET my_reaction=? WHERE chat_id=? AND id=?',
            [evt.emoji || null, evt.chat_id, evt.message_id], ['messages']);
        }
        break;
      }
      case 'typing':
        this.onEphemeral({ type: 'typing', chatId: evt.chat_id, userId: evt.user_id, name: evt.name });
        break;
      case 'chat':
        try {
          const c = await this.api.getChat(evt.chat_id);
          await this.upsertChatMeta(c);
        } catch {
          /* ignore */
        }
        break;
      default:
        break;
    }
  }

  private queueCatchUp(): void {
    if (this.cuTimer) return;
    this.cuTimer = setTimeout(() => {
      this.cuTimer = null;
      void this.catchUp();
    }, 300);
  }
}
