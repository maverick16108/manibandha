// Единая SQLite-схема мессенджера — одинаковая на всех платформах (web/RN/Flutter).
// Локальная БД — кэш/источник для UI; сервер авторитетен. Порядок сообщений — по seq.
export const SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS sync_state (
  key   TEXT PRIMARY KEY,
  value TEXT
);

CREATE TABLE IF NOT EXISTS chats (
  id               INTEGER PRIMARY KEY,
  type             TEXT NOT NULL,          -- direct | group
  title            TEXT,
  photo_url        TEXT,
  created_by       INTEGER,
  updated_at       TEXT,
  last_seq         INTEGER DEFAULT 0,      -- seq последнего сообщения
  my_last_read_seq INTEGER DEFAULT 0,      -- до какого seq прочитал я
  unread           INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS members (
  chat_id       INTEGER NOT NULL,
  user_id       INTEGER NOT NULL,
  full_name     TEXT,
  avatar_url    TEXT,
  role          TEXT,
  last_read_seq INTEGER DEFAULT 0,
  PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
  chat_id      INTEGER NOT NULL,
  client_uuid  TEXT NOT NULL,             -- идемпотентность + локальная идентичность
  id           INTEGER,                   -- серверный id (NULL пока не подтверждено)
  seq          INTEGER,                   -- серверный порядок (NULL для pending)
  author_id    INTEGER,
  author_name  TEXT,
  body         TEXT,
  reply_to_id  INTEGER,
  reply_preview TEXT,
  created_at   TEXT,
  edited_at    TEXT,
  edit_count   INTEGER DEFAULT 0,
  deleted      INTEGER DEFAULT 0,
  reactions    TEXT,                      -- JSON-агрегат [{emoji,count}]
  my_reaction  TEXT,                      -- эмодзи текущего пользователя
  status       TEXT DEFAULT 'sent',       -- pending | sent | failed
  local_ts     INTEGER,                   -- время локальной вставки (для сортировки pending)
  PRIMARY KEY (chat_id, client_uuid)
);
CREATE INDEX IF NOT EXISTS ix_messages_chat_seq ON messages(chat_id, seq);

CREATE TABLE IF NOT EXISTS outbox (
  client_uuid TEXT PRIMARY KEY,
  chat_id     INTEGER NOT NULL,
  body        TEXT,
  reply_to_id INTEGER,
  reply_quote TEXT,
  created_at  TEXT,
  attempts    INTEGER DEFAULT 0
);
`;

// Идемпотентные миграции для уже существующих локальных БД (ошибку «duplicate
// column» глотаем). Выполняются после SCHEMA_SQL при каждом старте.
export const MIGRATIONS = [
  'ALTER TABLE messages ADD COLUMN reactions TEXT',
  'ALTER TABLE messages ADD COLUMN my_reaction TEXT',
  'ALTER TABLE outbox ADD COLUMN reply_quote TEXT',
];
