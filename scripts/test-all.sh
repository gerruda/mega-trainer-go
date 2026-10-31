#!/usr/bin/env bash
#
# Runs integration and e2e tests against the local infrastructure described in
# .env (start it first with `make docker-up-dev`).
#
# The tests create their own temporary PostgreSQL schemas and delete them
# afterwards, so the developer database is never touched.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -f .env ]]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

: "${POSTGRES_HOST:?POSTGRES_HOST is required (see .env.example)}"
: "${POSTGRES_USER:?POSTGRES_USER is required (see .env.example)}"
: "${POSTGRES_DB:?POSTGRES_DB is required (see .env.example)}"
: "${REDIS_ADDR:?REDIS_ADDR is required (see .env.example)}"

export TEST_POSTGRES_HOST="$POSTGRES_HOST"
export TEST_POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export TEST_POSTGRES_USER="$POSTGRES_USER"
export TEST_POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
export TEST_POSTGRES_DB="$POSTGRES_DB"
export TEST_REDIS_ADDR="$REDIS_ADDR"
export TEST_REDIS_PASSWORD="${REDIS_PASSWORD:-}"
export TEST_POSTGRES_DSN="host=${TEST_POSTGRES_HOST} port=${TEST_POSTGRES_PORT} user=${TEST_POSTGRES_USER} password=${TEST_POSTGRES_PASSWORD} dbname=${TEST_POSTGRES_DB} sslmode=disable"

echo "integration tests (PostgreSQL ${TEST_POSTGRES_HOST}:${TEST_POSTGRES_PORT})"
go test -tags=integration -race -count=1 ./internal/storage/postgres/...

echo "e2e tests (fake Telegram API, PostgreSQL, Redis ${TEST_REDIS_ADDR})"
go test -tags=e2e -race -count=1 ./internal/app/...
