Пример logrotate конфига:

/var/log/bot/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 bot bot
}

Для работы нужно создать файл в корне проекта
env.env
и добавить туда переменные для окружения. Например:
BOT_TOKEN=1234567890
API_KEY=abcdefg1234567890

При разработке использовать команду для запуска инфраструктуры
docker-compose -f ./docker/docker-compose.dev.yml up -d

docker-compose -f ./docker/docker-compose.dev.yml --env-file .env up -d

# Просмотр данных
docker exec -it mega-trainer-bot-db env | grep BOT_PASSWORD
docker exec -it mega-trainer-bot-db psql -U bot_user -d mega-trainer-bot -c "SELECT * FROM users;"

# Бэкап базы
docker exec mega-trainer-bot-db pg_dump -U bot_user mega-trainer-bot > backup.sql

# Восстановление из бэкапа
docker exec -i mega-trainer-bot-db psql -U bot_user -d mega-trainer-bot < backup.sql

# Остановка и очистка
docker-compose -f docker-compose.dev.yml down
docker-compose -f docker-compose.dev.yml down -v  # с удалением данных
