#!/usr/bin/env bash
# Nightly PostgreSQL backup for Manibandha. Keeps the last 14 daily dumps.
# Install via cron:  0 3 * * *  /var/www/manibandha.prema.su/deploy/backup-db.sh
set -euo pipefail

BACKUP_DIR=/var/backups/manibandha
DB=manibandha
KEEP=14

mkdir -p "$BACKUP_DIR"
STAMP=$(date +%Y%m%d-%H%M%S)
FILE="$BACKUP_DIR/manibandha-$STAMP.sql.gz"

sudo -u postgres pg_dump "$DB" | gzip > "$FILE"

# prune old backups
ls -1t "$BACKUP_DIR"/manibandha-*.sql.gz | tail -n +$((KEEP + 1)) | xargs -r rm -f
echo "backup written: $FILE"
