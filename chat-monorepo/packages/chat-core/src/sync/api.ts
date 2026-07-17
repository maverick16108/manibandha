// REST-контракт мессенджера. Реализуется на каждой платформе поверх fetch/axios.
import type { Chat, ChatMessage, Contact, UpdatesResponse } from '../types.js';

export interface SendPayload {
  client_uuid: string;
  body: string;
  reply_to_id?: number | null;
  reply_quote?: string | null;
}

export interface CreateChatPayload {
  type: 'direct' | 'group';
  peer_id?: number;
  title?: string;
  member_ids?: number[];
}

export interface ReactResult {
  reactions: { emoji: string; count: number }[];
  my_reaction: string | null;
}

export interface ChatApi {
  listChats(): Promise<Chat[]>;
  getChat(id: number): Promise<Chat>;
  getUpdates(since: number, limit?: number): Promise<UpdatesResponse>;
  listMessages(chatId: number, beforeSeq?: number | null, limit?: number): Promise<ChatMessage[]>;
  send(chatId: number, payload: SendPayload): Promise<ChatMessage>;
  editMessage(chatId: number, messageId: number, body: string): Promise<ChatMessage>;
  deleteMessage(chatId: number, messageId: number): Promise<void>;
  markRead(chatId: number, seq: number): Promise<void>;
  createChat(payload: CreateChatPayload): Promise<Chat>;
  updateChat(chatId: number, payload: { title?: string; photo_url?: string | null }): Promise<Chat>;
  pin(chatId: number, pinned: boolean): Promise<void>;
  leaveChat(chatId: number): Promise<void>;
  react(chatId: number, messageId: number, emoji: string): Promise<ReactResult>;
  contacts(): Promise<Contact[]>;
}
