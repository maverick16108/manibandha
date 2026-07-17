// Синглтон ChatStore для мобилки + React-хук через useSyncExternalStore.
import { useSyncExternalStore } from 'react';
import { ChatStore, createReconnectingSocket, type ChatState } from '@manibandha/chat-core';
import { OpSqliteAdapter } from '@manibandha/adapter-op-sqlite';
import { chatApi } from './api';
import { API_BASE, getToken } from './config';

let store: ChatStore | null = null;

export async function initChatStore(meId: number): Promise<ChatStore> {
  if (store) return store;
  const db = new OpSqliteAdapter('manibandha-chat');
  await db.init();
  store = new ChatStore({
    db,
    api: chatApi,
    meId,
    makeSocket: (handlers) =>
      createReconnectingSocket({
        url: (t) => `${API_BASE.replace(/^http/, 'ws')}/api/ws/chat?token=${encodeURIComponent(t)}`,
        getToken,
        ...handlers,
      }),
  });
  await store.init();
  return store;
}

export function getStore(): ChatStore {
  if (!store) throw new Error('ChatStore not initialized — call initChatStore first');
  return store;
}

/** Реактивное состояние чата для компонентов. */
export function useChatState(): ChatState {
  const s = getStore();
  return useSyncExternalStore(s.subscribe, s.getSnapshot, s.getSnapshot);
}
