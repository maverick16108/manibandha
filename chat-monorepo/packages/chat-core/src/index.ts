// @manibandha/chat-core — общий local-first sync-слой мессенджера.
export * from './types.js';
export * from './schema.js';
export * from './db/adapter.js';
export * from './sync/api.js';
export * from './sync/socket.js';
export { ChatEngine } from './sync/engine.js';
export type { EngineOptions } from './sync/engine.js';
export { ChatStore } from './store.js';
export type { ChatState, ChatListItem, ChatStoreDeps } from './store.js';
