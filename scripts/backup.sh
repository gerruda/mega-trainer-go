#!/usr/bin/env bash
#
# Dumps the PostgreSQL database of the Docker Compose deployment to a gzip file
# and rotates old backups.
#
# Usage:
#   ./scripts/backup.sh
#
# Environment overrides:
#   BACKUP_DIR      where to store dumps            (default: ./backups)
#   RETENTION_DAYS  how many days to keep backups   (default: 14)
#   COMPOSE_FILE    compose file                    (default: docker/docker-compose.yml)
#   ENV_FILE        env file with POSTGRES_* values (default: .env)
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
COMPOSE_FILE="${COMPOSE_FILE:-docker/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-.env}"

if [[ ! -f "$COMPOSE_FILE" ]]; then
    echo "backup: compose file not found: $COMPOSE_FILE" >&2
    exit 1
fi
if [[ ! -f "$ENV_FILE" ]]; then
    echo "backup: env file not found: $ENV_FILE" >&2
    exit 1
fi

mkdir -p "$BACKUP_DIR"
timestamp="$(date +%Y%m%d-%H%M%S)"
target="${BACKUP_DIR}/mega-trainer-${timestamp}.sql.gz"

echo "backup: dumping database to ${target}"

# pg_dump runs inside the container, so its own POSTGRES_* environment provides
# the database name and user; nothing is interpolated on the host.
if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T postgres \
    sh -c 'pg_dump --no-owner --no-privileges -U "$POSTGRES_USER" "$POSTGRES_DB"' \
    | gzip -9 > "$target"; then
    echo "backup: pg_dump failed, removing incomplete file" >&2
    rm -f "$target"
    exit 1
fi

if [[ ! -s "$target" ]]; then
    echo "backup: dump is empty" >&2
    rm -f "$target"
    exit 1
fi

echo "backup: done ($(du -h "$target" | cut -f1))"

echo "backup: removing files older than ${RETENTION_DAYS} days"
find "$BACKUP_DIR" -name 'mega-trainer-*.sql.gz' -type f -mtime "+${RETENTION_DAYS}" -print -delete
