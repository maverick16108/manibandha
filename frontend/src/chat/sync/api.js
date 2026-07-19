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
  react: (chatId, messageId, emoji) => client.post(`/chats/${chatId}/messages/${messageId}/react`, { emoji }).then((r) => r.data),
  markRead: (chatId, seq) => client.post(`/chats/${chatId}/read`, { seq }),
  pin: (chatId, pinned) => client.post(`/chats/${chatId}/pin`, { pinned }),
  reorderPins: (ids) => client.post('/chats/pins/reorder', { ids }),
  pinMessage: (chatId, messageId) => client.post(`/chats/${chatId}/pin-message`, { message_id: messageId }),
  unpinMessage: (chatId) => client.post(`/chats/${chatId}/unpin-message`),
  leaveChat: (chatId) => client.delete(`/chats/${chatId}/leave`),
  createChat: (payload) => client.post('/chats', payload).then((r) => r.data),
  updateChat: (id, payload) => client.patch(`/chats/${id}`, payload).then((r) => r.data),
  contacts: () => client.get('/chats/contacts').then((r) => r.data),
  linkPreview: (url) => client.get('/link-preview', { params: { url } }).then((r) => r.data),
  searchMessages: (chatId, q) => client.get(`/chats/${chatId}/search`, { params: { q } }).then((r) => r.data),
  searchAllChats: (q) => client.get('/chats/search', { params: { q } }).then((r) => r.data),
  uploadsDims: (urls) => client.get('/uploads-dims', { params: { u: urls.join(',') } }).then((r) => r.data),
};
