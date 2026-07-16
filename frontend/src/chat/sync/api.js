// REST-обёртка мессенджера поверх общего axios-клиента. Легко подменяется в тестах.
import client from '../../api/client';

export const chatApi = {
  listChats: () => client.get('/chats').then((r) => r.data),
  getChat: (id) => client.get(`/chats/${id}`).then((r) => r.data),
  getUpdates: (since, limit = 300) => client.get('/chats/updates', { params: { since, limit } }).then((r) => r.data),
  listMessages: (chatId, beforeSeq, limit = 50) =>
    client.get(`/chats/${chatId}/messages`, { params: { before_seq: beforeSeq || undefined, limit } }).then((r) => r.data),
  send: (chatId, payload) => client.post(`/chats/${chatId}/messages`, payload).then((r) => r.data),
  editMessage: (chatId, messageId, body) => client.patch(`/chats/${chatId}/messages/${messageId}`, { body }).then((r) => r.data),
  deleteMessage: (chatId, messageId) => client.delete(`/chats/${chatId}/messages/${messageId}`),
  markRead: (chatId, seq) => client.post(`/chats/${chatId}/read`, { seq }),
  createChat: (payload) => client.post('/chats', payload).then((r) => r.data),
  contacts: () => client.get('/chats/contacts').then((r) => r.data),
};
