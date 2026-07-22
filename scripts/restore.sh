#!/bin/sh
set -e

usage() {
    echo "Usage: $0 <backup-file>"
    echo ""
    echo "Restore a PostgreSQL backup created by backup.sh"
    echo ""
    echo "Environment:"
    echo "  POSTGRES_HOST     (default: postgres)"
    echo "  POSTGRES_USER     (default: ft_hackthon)"
    echo "  POSTGRES_PASSWORD (default: ft_hackthon)"
    echo "  POSTGRES_DB       (default: ft_hackthon)"
    exit 1
}

BACKUP_FILE=$1
if [ -z "$BACKUP_FILE" ]; then
    usage
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: backup file not found: $BACKUP_FILE"
    exit 1
fi

echo "WARNING: This will overwrite the current database!"
echo "Restoring from: $BACKUP_FILE"
echo ""
echo "Press Ctrl+C within 5 seconds to cancel..."
sleep 5

echo "Dropping and recreating database..."
PGPASSWORD="${POSTGRES_PASSWORD:-ft_hackthon}" psql \
    -h "${POSTGRES_HOST:-postgres}" \
    -U "${POSTGRES_USER:-ft_hackthon}" \
    -d "${POSTGRES_DB:-ft_hackthon}" \
    -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

echo "Restoring from backup..."
gunzip -c "$BACKUP_FILE" | PGPASSWORD="${POSTGRES_PASSWORD:-ft_hackthon}" psql \
    -h "${POSTGRES_HOST:-postgres}" \
    -U "${POSTGRES_USER:-ft_hackthon}" \
    -d "${POSTGRES_DB:-ft_hackthon}"

echo "Restore complete!"
