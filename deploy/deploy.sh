#!/usr/bin/env bash
# Deploy / update Manibandha on patita.
#
# Прод работает на Go-бэкенде (systemd: manibandha-go, порт :8020).
# Схему БД по-прежнему ведёт Alembic (Python) — поэтому держим venv для миграций,
# а Python-сервис (manibandha-api, :8010) остаётся запущенным как мгновенный откат.
#
# Идемпотентно: git pull → миграции → сборка Go-бинаря → сборка фронта → рестарт.
set -euo pipefail

APP_DIR=/var/www/manibandha.prema.su
GO=${GO:-/usr/local/go/bin/go}
cd "$APP_DIR"

echo ">>> git pull"
git pull --ff-only

# Миграции схемы БД — Alembic (Python). Venv держим ради них и ради отката.
echo ">>> backend (Python) deps + migrations"
cd "$APP_DIR/backend"
.venv/bin/pip install -q -r requirements.txt
.venv/bin/alembic upgrade head

# Go-бэкенд — собираем бинарь (чистый Go, без cgo). cwebp нужен для пережатия фото.
echo ">>> build Go backend"
command -v cwebp >/dev/null 2>&1 || apt-get install -y -qq webp
cd "$APP_DIR/backend-go"
CGO_ENABLED=0 "$GO" build -o "$APP_DIR/mani-go" ./cmd/server
chown www-data:www-data "$APP_DIR/mani-go"
chmod +x "$APP_DIR/mani-go"

echo ">>> frontend build"
cd "$APP_DIR/frontend"
npm ci
npm run build

echo ">>> restart services + reload nginx"
systemctl restart manibandha-go             # боевой бэкенд (:8020)
systemctl restart manibandha-api || true    # Python (:8010) — idle, держим для отката
nginx -t && systemctl reload nginx

chown -R www-data:www-data "$APP_DIR"
echo ">>> done (backend: Go on :8020)"
