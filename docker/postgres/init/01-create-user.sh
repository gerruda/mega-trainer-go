#!/bin/sh
set -e

echo "Running database initialization..."

# Используем переменные окружения
BOT_USER="${BOT_USER}"
BOT_PASSWORD="${BOT_PASSWORD}"


# Ждем пока PostgreSQL будет готов
until pg_isready -U "$POSTGRES_USER"; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

echo "PostgreSQL is ready. Creating bot user..."

# Выполняем SQL команды
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    \\echo 'Creating user $BOT_USER...'

    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$BOT_USER') THEN
            CREATE USER $BOT_USER WITH PASSWORD '$BOT_PASSWORD';
            RAISE NOTICE 'User $BOT_USER created';
        ELSE
            ALTER USER $BOT_USER WITH PASSWORD '$BOT_PASSWORD';
            RAISE NOTICE 'User $BOT_USER password updated';
        END IF;
    END
    \$\$;

    -- Даем права
    ALTER USER $BOT_USER WITH SUPERUSER;
    GRANT ALL PRIVILEGES ON DATABASE $POSTGRES_DB TO $BOT_USER;

    \\echo 'User setup completed';
EOSQL

echo "Database initialization completed successfully!"
