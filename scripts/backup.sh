#!/bin/sh
set -e

BACKUP_DIR=${BACKUP_DIR:-/backups}
RETENTION_DAYS=${RETENTION_DAYS:-7}
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILENAME="ft_hackthon_${TIMESTAMP}.sql.gz"

mkdir -p "$BACKUP_DIR"

echo "Backing up PostgreSQL to $BACKUP_DIR/$FILENAME ..."
PGPASSWORD="${POSTGRES_PASSWORD:-ft_hackthon}" pg_dump \
    -h "${POSTGRES_HOST:-postgres}" \
    -U "${POSTGRES_USER:-ft_hackthon}" \
    -d "${POSTGRES_DB:-ft_hackthon}" \
    --clean \
    --if-exists \
    | gzip > "$BACKUP_DIR/$FILENAME"

echo "Backup complete: $(ls -lh "$BACKUP_DIR/$FILENAME" | awk '{print $5}')"

# Prune old backups
find "$BACKUP_DIR" -name "ft_hackthon_*.sql.gz" -mtime +"$RETENTION_DAYS" -delete
echo "Pruned backups older than $RETENTION_DAYS days"
