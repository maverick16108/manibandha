// Sync-движок мессенджера (framework-agnostic). Сервер — источник истины.
//
// Отправка: оптимистично пишем message(status=pending) + outbox, показываем сразу;
//   POST идемпотентен по client_uuid; после ACK — серверные id/seq, status=sent.
// Приём: WS отдаёт апдейты (применяем идемпотентным upsert по client_uuid);
//   курсор pts двигаем только догоном GET /updates?since=pts (не теряем «потерянный» бродкаст).
// Реконнект: catchUp() добирает пропущенное, flushOutbox() дошлёт неотправленное.

const MAX_ATTEMPTS = 5;

export class ChatEngine {
  constructor({ db, api, meId, onEphemeral, genUuid, now }) {
    this.db = db;
    this.api = api;
    this.meId = meId;
    this.onEphemeral = onEphemeral || (() => {});
    this._genUuid = genUuid || (() => globalThis.crypto.randomUUID());
    this._now = now || (() => Date.now());
    this._catchUpQueued = false;
    this._flushing = false;
  }

  // ── sync_state / pts ──────────────────────────────────────────────────
  async _getPts() {
    const row = await this.db.get("SELECT value FROM sync_state WHERE key='pts'");
    return row ? Number(row.value) || 0 : 0;
  }
  async _setPts(v) {
    await this.db.run(
      "INSERT INTO sync_state(key,value) VALUES('pts',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
      [String(v)],
    );
  }

  // ── запись чатов/сообщений в локальную БД ─────────────────────────────
  async upsertChatMeta(chat) {
    const items = [{
      sql: `INSERT INTO chats(id,type,title,photo_url,created_by,updated_at,last_seq,my_last_read_seq,unread,pinned,pin_order)
            VALUES(?,?,?,?,?,?,?,?,?,?,?)
            ON CONFLICT(id) DO UPDATE SET type=excluded.type, title=excluded.title,
              photo_url=excluded.photo_url, created_by=excluded.created_by, updated_at=excluded.updated_at,
              pinned=excluded.pinned, pin_order=excluded.pin_order`,
      params: [
        chat.id, chat.type, chat.title || null, chat.photo_url || null, chat.created_by || null,
        chat.updated_at || null, chat.last_message?.seq || 0, myLastRead(chat, this.meId), chat.unread || 0,
        chat.pinned ? 1 : 0, chat.pin_order || 0,
      ],
    }];
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
    if (chat.last_message) await this._writeMessage(chat.last_message, false);
    await this._recomputeUnread(chat.id);
  }

  async _ensureChat(chatId) {
    const c = await this.db.get('SELECT id FROM chats WHERE id=?', [chatId]);
    if (!c) {
      try {
        const chat = await this.api.getChat(chatId);
        await this.upsertChatMeta(chat);
      } catch { /* нет доступа/сети — сообщение всё равно сохраним */ }
    }
  }

  async _writeMessage(m, emit = true) {
    const uuid = m.client_uuid || `srv:${m.id}`;
    const localTs = m.created_at ? Date.parse(m.created_at) || 0 : this._now();
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
    // авторитетное сообщение пришло — убрать из очереди отправки
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

  async _applyServerMessage(m) {
    await this._ensureChat(m.chat_id);
    await this._writeMessage(m, true);
    await this._recomputeUnread(m.chat_id);
  }

  async _recomputeUnread(chatId) {
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
  async bootstrap() {
    try {
      const chats = await this.api.listChats();
      for (const c of chats) await this.upsertChatMeta(c);
    } catch (e) { console.warn('[chat] listChats failed', e); }
    await this.catchUp();
    await this.flushOutbox();
  }

  async catchUp() {
    if (this._catchUpQueued) return;
    this._catchUpQueued = true;
    try {
      let pts = await this._getPts();
      for (let i = 0; i < 200; i++) {
        let resp;
        try { resp = await this.api.getUpdates(pts); } catch { break; }
        const ups = resp.updates || [];
        // pts двигаем ТОЛЬКО за успешно применёнными апдейтами — если запись упала,
        // курсор не проскакивает пропущенное сообщение (иначе — вечная дыра/расхождение)
        let applied = pts;
        let ok = true;
        for (const u of ups) {
          if (u.type === 'message' && u.message) {
            try { await this._applyServerMessage(u.message); }
            catch { ok = false; break; }
          }
          const s = Number(u.seq) || 0;
          if (s > applied) applied = s;
        }
        const np = ok ? (Number(resp.pts) || applied) : applied;
        if (np <= pts) break;
        pts = np;
        await this._setPts(pts);
        if (!ok || !resp.has_more) break;
      }
    } finally {
      this._catchUpQueued = false;
    }
  }

  // сигнатура хвоста чата — чтобы сверка не дёргала перерисовку, когда ничего не изменилось
  async _chatSig(chatId) {
    const r = await this.db.get(
      `SELECT COUNT(*) c, COALESCE(MAX(seq),0) s, COALESCE(SUM(edit_count),0) e,
              COALESCE(SUM(deleted),0) d FROM messages WHERE chat_id=?`, [chatId]);
    return r ? `${r.c}:${r.s}:${r.e}:${r.d}` : '';
  }

  // Сверить хвост чата с сервером (авторитетно). Вызывается при открытии, реконнекте,
  // возврате фокуса/сети и периодически — гарантирует, что хвост совпадает с сервером.
  // Перерисовку сигналим только если данные реально изменились.
  async ensureChatMessages(chatId, beforeSeq) {
    try {
      const before = beforeSeq ? null : await this._chatSig(chatId);
      const msgs = await this.api.listMessages(chatId, beforeSeq, 50);
      for (const m of msgs) await this._writeMessage(m, false);
      await this._recomputeUnread(chatId);
      const changed = beforeSeq ? true : (before !== await this._chatSig(chatId));
      if (changed) await this.db.run("UPDATE sync_state SET value=value WHERE key='pts'", [], ['messages']);
      return msgs.length;
    } catch { return 0; }
  }

  async send(chatId, body, replyToId = null, replyQuote = null) {
    const text = (body || '').trim();
    if (!text) return null;
    const uuid = this._genUuid();
    const ts = this._now();
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
    this.flushOutbox();
    return uuid;
  }

  async updateChat(chatId, payload) {
    const c = await this.api.updateChat(chatId, payload);
    await this.upsertChatMeta(c);
  }

  async pinChat(chatId, pinned) {
    await this.api.pin(chatId, pinned);
    await this.db.run('UPDATE chats SET pinned=? WHERE id=?', [pinned ? 1 : 0, chatId], ['chats']);
  }

  async leaveChat(chatId) {
    await this.api.leaveChat(chatId);
    await this.db.batch([
      { sql: 'DELETE FROM messages WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM members WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM outbox WHERE chat_id=?', params: [chatId] },
      { sql: 'DELETE FROM chats WHERE id=?', params: [chatId] },
    ], ['chats', 'messages']);
  }

  async flushOutbox() {
    if (this._flushing) return;
    this._flushing = true;
    // дренируем ПОКА в очереди есть новые строки: сообщение, добавленное во время
    // отправки, иначе «залипло» бы pending до следующего триггера (быстрый ввод).
    // attempted не даёт повторно крутить одну и ту же неотправленную (упавшую) строку.
    const attempted = new Set();
    try {
      for (;;) {
        const rows = await this.db.all('SELECT * FROM outbox ORDER BY created_at ASC, rowid ASC');
        const fresh = rows.filter((r) => !attempted.has(r.client_uuid));
        if (!fresh.length) break;
        for (const row of fresh) { attempted.add(row.client_uuid); await this._flushOne(row); }
      }
    } finally {
      this._flushing = false;
    }
  }

  async _flushOne(row) {
    try {
      const m = await this.api.send(row.chat_id, {
        client_uuid: row.client_uuid, body: row.body, reply_to_id: row.reply_to_id || null, reply_quote: row.reply_quote || null,
      });
      await this._writeMessage(m, true);       // проставит id/seq/status=sent и удалит из outbox
      await this._recomputeUnread(row.chat_id);
    } catch (e) {
      const attempts = (row.attempts || 0) + 1;
      const status = attempts >= MAX_ATTEMPTS ? 'failed' : 'pending';
      await this.db.run('UPDATE outbox SET attempts=? WHERE client_uuid=?', [attempts, row.client_uuid], []);
      await this.db.run('UPDATE messages SET status=? WHERE chat_id=? AND client_uuid=?',
        [status, row.chat_id, row.client_uuid], ['messages']);
    }
  }

  async retryFailed() {
    await this.db.run("UPDATE outbox SET attempts=0", [], []);
    await this.db.run("UPDATE messages SET status='pending' WHERE status='failed'", [], ['messages']);
    await this.flushOutbox();
  }

  async markRead(chatId, seq) {
    if (!seq) return;
    await this.db.run(
      'UPDATE chats SET my_last_read_seq=MAX(my_last_read_seq,?), unread=0 WHERE id=?',
      [seq, chatId], ['chats'],
    );
    await this.db.run('UPDATE members SET last_read_seq=MAX(last_read_seq,?) WHERE chat_id=? AND user_id=?',
      [seq, chatId, this.meId], ['members']);
    try { await this.api.markRead(chatId, seq); } catch { /* дошлём позже */ }
  }

  async editMessage(chatId, messageId, body) {
    const m = await this.api.editMessage(chatId, messageId, body);
    await this._writeMessage(m, true);
  }

  async deleteMessage(chatId, messageId) {
    await this.api.deleteMessage(chatId, messageId);
    await this.db.run('UPDATE messages SET deleted=1, body=\'\' WHERE chat_id=? AND id=?', [chatId, messageId], ['messages']);
  }

  // «удалить для себя» — скрываем локально, на сервер не ходим
  async hideMessage(chatId, messageId) {
    await this.db.run('UPDATE messages SET hidden=1 WHERE chat_id=? AND id=?', [chatId, messageId], ['messages']);
  }

  async react(chatId, messageId, emoji) {
    // оптимистично: мгновенно показываем свою реакцию, потом синхронизируем с сервером
    const row = await this.db.get('SELECT reactions, my_reaction FROM messages WHERE chat_id=? AND id=?', [chatId, messageId]);
    if (row) {
      let list = [];
      try { list = JSON.parse(row.reactions || '[]'); } catch { list = []; }
      const prev = row.my_reaction || null;
      const dec = (em) => {
        const it = list.find((r) => r.emoji === em);
        if (it) { it.count = (it.count || 1) - 1; if (it.count <= 0) list = list.filter((r) => r.emoji !== em); }
      };
      if (prev) dec(prev);                 // снять прошлый голос
      let mine = null;
      if (prev !== emoji) {                // не тот же смайл — ставим новый
        mine = emoji;
        const it = list.find((r) => r.emoji === emoji);
        if (it) it.count = (it.count || 0) + 1; else list.push({ emoji, count: 1 });
      }
      await this.db.run('UPDATE messages SET reactions=?, my_reaction=? WHERE chat_id=? AND id=?',
        [JSON.stringify(list), mine, chatId, messageId], ['messages']);
    }
    try {
      const res = await this.api.react(chatId, messageId, emoji);  // { reactions, my_reaction }
      await this.db.run('UPDATE messages SET reactions=?, my_reaction=? WHERE chat_id=? AND id=?',
        [JSON.stringify(res.reactions || []), res.my_reaction || null, chatId, messageId], ['messages']);
    } catch { /* сервер поправит на следующей сверке */ }
  }

  async createChat(payload) {
    const c = await this.api.createChat(payload);
    await this.upsertChatMeta(c);
    return c.id;
  }

  // ── обработка WS-апдейтов ───────────────────────────────────────────────
  async handleWs(evt) {
    switch (evt.type) {
      case 'message':
        if (evt.message) {
          await this._applyServerMessage(evt.message);
          this._queueCatchUp(); // сдвинуть pts (закрыть возможные пропуски)
        }
        break;
      case 'edit':
        if (evt.message) await this._writeMessage(evt.message, true);
        break;
      case 'delete':
        await this.db.run('UPDATE messages SET deleted=1, body=\'\' WHERE chat_id=? AND id=?',
          [evt.chat_id, evt.message_id], ['messages']);
        break;
      case 'read':
        await this.db.run('UPDATE members SET last_read_seq=MAX(last_read_seq,?) WHERE chat_id=? AND user_id=?',
          [evt.last_read_seq, evt.chat_id, evt.user_id], ['members']);
        break;
      case 'react': {
        const mine = evt.user_id === this.meId;
        if (mine) {
          await this.db.run('UPDATE messages SET reactions=?, my_reaction=? WHERE chat_id=? AND id=?',
            [JSON.stringify(evt.reactions || []), evt.emoji || null, evt.chat_id, evt.message_id], ['messages']);
        } else {
          await this.db.run('UPDATE messages SET reactions=? WHERE chat_id=? AND id=?',
            [JSON.stringify(evt.reactions || []), evt.chat_id, evt.message_id], ['messages']);
        }
        break;
      }
      case 'typing':
        this.onEphemeral({ type: 'typing', chatId: evt.chat_id, userId: evt.user_id, name: evt.name });
        break;
      case 'chat': {
        try { const c = await this.api.getChat(evt.chat_id); await this.upsertChatMeta(c); } catch { /* ignore */ }
        break;
      }
      default: break;
    }
  }

  _queueCatchUp() {
    // микродебаунс, чтобы не звать /updates на каждое WS-сообщение
    if (this._cuTimer) return;
    this._cuTimer = setTimeout(() => { this._cuTimer = null; this.catchUp(); }, 300);
  }
}

function myLastRead(chat, meId) {
  const me = (chat.members || []).find((m) => m.user_id === meId);
  return me ? me.last_read_seq || 0 : 0;
}
