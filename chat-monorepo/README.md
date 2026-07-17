# Manibandha Chat — monorepo

Local-first мессенджер (веб + мобилка) с общим синк-ядром.

```
chat-monorepo/
├─ packages/
│  ├─ chat-core/            # @manibandha/chat-core — вся логика, без UI и без привязки к платформе
│  │  ├─ types.ts           # доменные типы (Chat, ChatMessage, ChatMember, ChatUpdate…)
│  │  ├─ schema.ts          # SCHEMA_SQL + MIGRATIONS + schemaStatements()
│  │  ├─ db/adapter.ts      # DbAdapter — единый интерфейс к SQLite (init/exec/run/all/get/batch/subscribe)
│  │  ├─ sync/api.ts        # ChatApi — HTTP-контракт с бэком
│  │  ├─ sync/socket.ts     # ChatSocket + createReconnectingSocket()
│  │  ├─ sync/engine.ts     # ChatEngine — bootstrap, catch-up по pts, outbox, реалтайм по WS
│  │  └─ store.ts           # ChatStore — наблюдаемое состояние (subscribe/getSnapshot)
│  └─ adapter-op-sqlite/    # @manibandha/adapter-op-sqlite — DbAdapter поверх @op-engineering/op-sqlite (RN)
└─ apps/
   └─ mobile/               # Expo (React Native, New Architecture) — тонкий UI поверх chat-core
```

## Идея

Вся синхронизация (локальная БД, outbox, catch-up по `pts`, идемпотентность по `client_uuid`,
реалтайм по WebSocket) живёт в `@manibandha/chat-core` и **не зависит от платформы**.
БД спрятана за интерфейсом `DbAdapter`, а UI — за наблюдаемым `ChatStore`.

- **Web** подставляет адаптер на `wa-sqlite` + OPFS (в вебовом приложении Vue).
- **Mobile** подставляет `@manibandha/adapter-op-sqlite` (нативный SQLite через JSI).

Один и тот же движок → один источник багов и фич, а не два.

## Запуск

```bash
pnpm install

# типы/сборка ядра
pnpm --filter @manibandha/chat-core build

# мобилка (нужен dev-client, НЕ Expo Go — op-sqlite это нативный модуль)
pnpm --filter @manibandha/mobile ios      # или android
```

> op-sqlite — нативный модуль, поэтому Expo Go не подойдёт: собирайте dev-client
> (`expo run:ios` / `expo run:android`) или EAS-билд. Плагин уже прописан в `app.json`.

Для быстрой проверки в мобилке используется демо-вход: вставьте `token` (можно взять из
`localStorage` веб-приложения) и свой `user id`. В проде — тот же SMS-логин, что и на сайте.

## Бэкенд

Общий FastAPI-бэк (эндпоинты `/api/chats*`, `/api/chats/updates?since=pts`, WS `/api/ws/chat`)
живёт в основном репозитории проекта. Контракт зафиксирован в `chat-core/src/sync/api.ts`.
