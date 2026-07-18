#!/usr/bin/env bash
# Deploy / update Manibandha on patita.
#
# Прод полностью на Go (systemd: manibandha-go, порт :8020). Python удалён.
# Миграции схемы БД применяет сам Go-бинарь при старте (internal/migrate).
# .env и uploads лежат в корне приложения ($APP_DIR), не в репозитории.
#
# Идемпотентно: git pull → сборка Go-бинаря → сборка фронта → рестарт.
set -euo pipefail

APP_DIR=/var/www/manibandha.prema.su
GO=${GO:-/usr/local/go/bin/go}
cd "$APP_DIR"

echo ">>> git pull"
git pull --ff-only

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

echo ">>> restart service + reload nginx"
systemctl restart manibandha-go          # боевой бэкенд (:8020); миграции применит при старте
nginx -t && systemctl reload nginx

# uploads (в корне) не трогаем chown-ом рекурсивно ниже по ошибке — они уже www-data
chown -R www-data:www-data "$APP_DIR/mani-go" "$APP_DIR/backend-go" "$APP_DIR/frontend/dist"
echo ">>> done (backend: Go on :8020)"
