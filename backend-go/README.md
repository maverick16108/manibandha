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
backend-go/
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
cd backend-go
ENV_FILE=../backend/.env go run ./cmd/server   # берёт те же SECRET_KEY/DATABASE_URL
# порт по умолчанию 8010 (как у Python-дев-сервера)
go test ./...
```

## Статус портирования

Готово (foundation + auth vertical slice):
- [x] конфиг, подключение к БД, статика `/uploads`
- [x] JWT + пароли (совместимость с Python проверена тестами)
- [x] `POST /api/auth/login`, `/auth/phone/request`, `/auth/phone/verify` (вход+регистрация)
- [x] `POST /auth/refresh`, `GET/PATCH /auth/me`
- [x] capabilities: `GET /api/me/capabilities`, `GET /api/capabilities`
- [x] `GET /api/health`

Осталось портировать (по модулям, каждый — свой файл в `internal/web`):
- [ ] users (CRUD, `/me/capabilities` уже есть)
- [ ] roles (CRUD ролей, назначение)
- [ ] disciples (+ scope по ролям), pipeline, disciple notes/files
- [ ] threads (вопросы/отчёты) + лайки
- [ ] forum (секции/темы/посты/реакции)
- [ ] chat (REST + **WebSocket** + local-first sync `/updates?since=pts`)
- [ ] uploads (пережатие в webp + превью — портировать на Go image libs)
- [ ] conferences (LiveKit Go SDK), recordings, bans
- [ ] events, drafts, settings, dictionaries (cities/regions/countries/temples)
- [ ] reports (экспорт PDF/Excel — заменить reportlab/openpyxl на Go-аналоги)
- [ ] стартовый seed ролей/гуру (перенести из Python bootstrap)

## Переключение на проде (позже, когда порт будет полным)

Собрать бинарь, поднять как systemd-сервис на другом порту, прогнать против
staging-БД, затем переключить `upstream` в nginx с Python на Go. Python-версия
остаётся в репозитории как откат.
