// Доменные типы мессенджера — совпадают со схемами бэкенда (ChatOut / ChatMessageOut).
export type ChatType = 'direct' | 'group';
export type MessageStatus = 'pending' | 'sent' | 'failed';

export interface Reaction {
  emoji: string;
  count: number;
}

export interface ChatMessage {
  id?: number | null;
  chat_id: number;
  seq?: number | null;
  client_uuid: string;
  author_id?: number | null;
  author_name?: string | null;
  body: string;
  reply_to_id?: number | null;
  reply_preview?: string | null;
  created_at?: string | null;
  edited_at?: string | null;
  edit_count?: number;
  deleted?: boolean;
  reactions?: Reaction[];
  my_reaction?: string | null;
  // локальные поля
  status?: MessageStatus;
  local_ts?: number;
  hidden?: boolean;
}

export interface ChatMember {
  chat_id?: number;
  user_id: number;
  full_name?: string | null;
  avatar_url?: string | null;
  role?: string;
  last_read_seq?: number;
}

export interface Chat {
  id: number;
  type: ChatType;
  title?: string | null;
  photo_url?: string | null;
  created_by?: number | null;
  created_at?: string;
  updated_at?: string | null;
  members?: ChatMember[];
  last_message?: ChatMessage | null;
  unread?: number;
  pinned?: boolean;
}

export interface ChatUpdate {
  type: string;
  seq: number;
  chat_id: number;
  message?: ChatMessage | null;
  message_id?: number | null;
}

export interface UpdatesResponse {
  updates: ChatUpdate[];
  pts: number;
  has_more: boolean;
}

export interface Contact {
  id: number;
  full_name?: string | null;
  avatar_url?: string | null;
  role?: string | null;
}

/** Событие «печатает…» / статус соединения — не сохраняется в БД. */
export type EphemeralEvent =
  | { type: 'typing'; chatId: number; userId: number; name?: string }
  | { type: 'connection'; status: 'online' | 'offline' };
