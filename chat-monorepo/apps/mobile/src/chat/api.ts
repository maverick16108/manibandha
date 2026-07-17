// Реализация ChatApi поверх fetch (глобальный в React Native).
import type { ChatApi } from '@manibandha/chat-core';
import { API_BASE, getToken } from './config';

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const t = getToken();
  const res = await fetch(`${API_BASE}/api${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(t ? { Authorization: `Bearer ${t}` } : {}),
      ...(init.headers || {}),
    },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  if (res.status === 204) return undefined as unknown as T;
  return (await res.json()) as T;
}

export const chatApi: ChatApi = {
  listChats: () => req('/chats'),
  getChat: (id) => req(`/chats/${id}`),
  getUpdates: (since, limit = 300) => req(`/chats/updates?since=${since}&limit=${limit}`),
  listMessages: (chatId, beforeSeq, limit = 50) =>
    req(`/chats/${chatId}/messages?${beforeSeq ? `before_seq=${beforeSeq}&` : ''}limit=${limit}`),
  send: (chatId, payload) => req(`/chats/${chatId}/messages`, { method: 'POST', body: JSON.stringify(payload) }),
  editMessage: (chatId, mid, body) => req(`/chats/${chatId}/messages/${mid}`, { method: 'PATCH', body: JSON.stringify({ body }) }),
  deleteMessage: (chatId, mid) => req(`/chats/${chatId}/messages/${mid}`, { method: 'DELETE' }),
  markRead: (chatId, seq) => req(`/chats/${chatId}/read`, { method: 'POST', body: JSON.stringify({ seq }) }),
  createChat: (p) => req('/chats', { method: 'POST', body: JSON.stringify(p) }),
  updateChat: (chatId, p) => req(`/chats/${chatId}`, { method: 'PATCH', body: JSON.stringify(p) }),
  pin: (chatId, pinned) => req(`/chats/${chatId}/pin`, { method: 'POST', body: JSON.stringify({ pinned }) }),
  leaveChat: (chatId) => req(`/chats/${chatId}/leave`, { method: 'DELETE' }),
  react: (chatId, mid, emoji) => req(`/chats/${chatId}/messages/${mid}/react`, { method: 'POST', body: JSON.stringify({ emoji }) }),
  contacts: () => req('/chats/contacts'),
};
