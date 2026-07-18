# Manibandha — Go backend (port)

Порт бэкенда с FastAPI (Python) на Go. Цель — **drop-in замена**: тот же
PostgreSQL (та же схема, миграции остаются на Alembic), тот же `SECRET_KEY` и
формат JWT, те же пути `/api/...`. Фронтенд менять не нужно.

## Почему это работает без миграции данных

- **JWT**: HS256, payload `{sub=email, role, exp}` — идентично `app/core/security.py`.
  Проверено тестом: Go парсит токен, выданный PyJWT, с тем же секретом.
- **Пароли**: `passlib pbkdf2_sha256`. Go умеет проверять существующие хеши
  (формат `$pbkdf2-sha256$rounds$salt$checksum`, ab64). Проверено тестом.
- **БД**: подключение к тому же Postgres (GORM/pgx), модели маппятся на
  существующие таблицы. Схему и миграции продолжает вести Alembic (Python).

## Стек

- `go-chi/chi` — роутер, `go-chi/cors`
- `gorm.io/gorm` + `driver/postgres` — доступ к БД
- `golang-jwt/jwt/v5`, `golang.org/x/crypto/pbkdf2`

## Структура

```
backend/
├── cmd/server/main.go        # точка входа
└── internal/
    ├── config/               # env (.env), зеркалит app/core/config.py
    ├── database/             # подключение к тому же Postgres
    ├── models/               # GORM-структуры на существующие таблицы
    ├── security/             # JWT + passlib pbkdf2 (+ тесты совместимости)
    ├── caps/                 # каталог прав + has_cap (из capabilities.py)
    └── web/                  # хендлеры, middleware, роутер
```

## Запуск

```bash
cd backend
ENV_FILE=../backend/.env go run ./cmd/server   # берёт те же SECRET_KEY/DATABASE_URL
# порт по умолчанию 8010 (как у Python-дев-сервера)
go test ./...
```

## Статус портирования — ГОТОВО ✅

Все модули портированы и **сверены на паритет** с Python (одна и та же локальная БД,
одинаковые запросы, сравнение JSON байт-в-байт). Проверка маршрутов: **все 130
эндпоинтов Python присутствуют в Go**.

- [x] foundation: конфиг, БД (GORM/pgx, timestamptz→UTC), JWT+пароли (совместимость с Python — тесты)
- [x] auth (login, phone request/verify+регистрация, refresh, me), capabilities
- [x] users, roles (CRUD, назначение ролей)
- [x] dictionaries (cities/regions/countries/temples)
- [x] disciples (список+фильтры+скоуп+сортировка, CRUD, approve, notes, files) + pipeline + mentors
- [x] threads (вопросы/отчёты/approval, реакции, nav-counts, stats)
- [x] events (+публичный календарь), drafts, settings
- [x] forum (разделы/темы/посты, реакции с who/mine, участники, модерация)
- [x] chat — REST + **WebSocket** (realtime + typing) + sync `/updates?since=pts`
- [x] uploads — пережатие в webp + превью (через `cwebp`)
- [x] conferences + recordings + bans (LiveKit: минт токенов, Twirp RoomService/Egress, webhook)
- [x] reports — агрегаты + экспорт xlsx (excelize) / pdf (fpdf + встроенный DejaVuSans)

### Что нельзя проверить локально
- **conferences**: вызовы LiveKit (RoomService/Egress) требуют живого LiveKit-сервера.
  Проверено: минт токена совпадает с Python (decoded JWT claims идентичны), CRUD — паритет.
- **pdf**: бинарь не байт-в-байт с reportlab (другая библиотека), но валиден и содержит
  тот же контент с корректной кириллицей.

## Внешние зависимости на хосте
- `cwebp` (libwebp-tools) — для пережатия изображений в uploads.
- LiveKit-сервер + env `LIVEKIT_API_KEY/SECRET/URL` — для конференций (как и в Python).

## Bootstrap (миграции/seed) остаётся на Python
Схема БД ведётся Alembic (Python), поэтому миграции и разовые скрипты
(`python -m app.seed`, `python -m app.create_admin`, `seed_roles`) остаются в Python —
их всё равно надо запускать при развёртывании. Go-сервер — это **рантайм API**,
работающий с уже подготовленной БД.

## Переключение на проде
Собрать бинарь (`go build ./cmd/server`), поднять systemd-сервисом на отдельном порту,
прогнать против staging-БД (Alembic-миграции применяет Python), затем переключить
`upstream` в nginx с Python на Go. Python-версия остаётся в репозитории как откат.
