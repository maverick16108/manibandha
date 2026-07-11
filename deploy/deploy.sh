#!/usr/bin/env bash
# Deploy / update Manibandha on patita.
# Idempotent: pulls latest code, updates deps, runs migrations, rebuilds frontend, restarts API.
set -euo pipefail

APP_DIR=/var/www/manibandha.prema.su
cd "$APP_DIR"

echo ">>> git pull"
git pull --ff-only

echo ">>> backend deps + migrations"
cd "$APP_DIR/backend"
.venv/bin/pip install -q -r requirements.txt
.venv/bin/alembic upgrade head

echo ">>> frontend build"
cd "$APP_DIR/frontend"
npm ci
npm run build

echo ">>> restart API + reload nginx"
systemctl restart manibandha-api
nginx -t && systemctl reload nginx

chown -R www-data:www-data "$APP_DIR"
echo ">>> done"
