# Manibandha — учёт учеников

Веб-приложение для учёта учеников Е.М. Манибандха Прабху (инициирующего гуру ИСККОН):
карточки учеников, статусы инициации, путь аспиранта (пайплайн), поиск/фильтры, отчёты и экспорт.

**Стек:** FastAPI + SQLAlchemy + Alembic + PostgreSQL (бэкенд) · Vue 3 + Vite + Tailwind (фронт).

## Структура

```
backend/      FastAPI: модели, схемы, роуты, миграции Alembic, seed
frontend/     Vue 3 SPA: публичная главная, вход, кабинет (ученики/храмы/отчёты/пользователи)
deploy/       systemd unit, nginx config, deploy.sh, backup-db.sh
docker-compose.yml   Локальный PostgreSQL
```

## Роли

- **Гуру** — видит всех, полный доступ, управление пользователями.
- **Секретарь** — ведёт данные, отчёты.
- **Наставник (куратор)** — видит закреплённых учеников, редактирует их пайплайн.
- **Ученик** — видит/обновляет свою анкету (опционально).

## Локальная разработка

```bash
# 1. PostgreSQL
docker compose up -d db

# 2. Бэкенд
cd backend
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env
alembic upgrade head
python -m app.seed --demo           # создаёт гуру + демо-данные
uvicorn app.main:app --reload --port 8010

# 3. Фронтенд (в другом терминале)
cd frontend
npm install
npm run dev                          # http://localhost:5173 (проксирует /api на :8010)
```

Первый вход: `guru@manibandha.local` / `change-me` (см. `.env`).

## Фотографии гуру

Реальные фото не хранятся в git. Положите `1.jpg`–`4.jpg` в `frontend/public/guru/`
(1 — портрет для hero и страницы входа, 2–4 — галерея). Отображаются в ч/б автоматически.

## Деплой (patita → https://manibandha.prema.su)

Схема: nginx отдаёт `frontend/dist`, проксирует `/api` на gunicorn/uvicorn (127.0.0.1:8010),
PostgreSQL нативно на сервере, автообновление сертификата — certbot. См. `deploy/`.

Обновление: `bash deploy/deploy.sh` на сервере.
