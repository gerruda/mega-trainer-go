# Эксплуатация

Как запускать, обновлять и обслуживать бота.

## Режимы запуска

| Режим | Как | Когда |
|---|---|---|
| Локально | `make docker-up-dev` + `make run` | разработка |
| Полный Docker | `make docker-up` | быстрый стенд, VPS |
| systemd | `deployments/systemd/bot.service`, `cron.service` | прод на хосте |

`make run` собирает `bin/bot`, `bin/cron`, `bin/migrate`, применяет миграции и
запускает бота.

## Обязательные переменные окружения

`.env` (см. `.env.example`):

| Переменная | Назначение |
|---|---|
| `BOT_TOKEN` | токен от @BotFather |
| `POSTGRES_HOST` / `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | БД |
| `REDIS_ADDR` | состояние диалогов |
| `TELEGRAM_PROXY_URL` | HTTP(S)-прокси, если Telegram заблокирован |
| `REDIS_PASSWORD`, `LOG_LEVEL` | опционально |

Проверка конфига: `./bin/bot` без переменных завершится со списком всех
недостающих параметров и кодом ≠ 0.

## Миграции

```bash
make migrate-up      # применить
make migrate-status  # статус
make migrate-down    # откатить последнюю
```

Идемпотентно, защищено advisory lock. В Docker применяет сервис `migrate`
(до старта бота), в systemd — `ExecStartPre`.

## Напоминания

`cmd/cron` — отдельный процесс, раз в `reminders.interval_hours` (по умолчанию 24 ч)
рассылает активным пользователям напоминание записать вес.

```bash
make run-cron                 # локально
systemctl enable --now mega-trainer-cron   # на хосте
```

Отключается в конфиге: `reminders.enabled: false`.

## Бэкапы

```bash
make backup
# или
BACKUP_DIR=/var/backups/mega-trainer RETENTION_DAYS=30 ./scripts/backup.sh
```

- делает `pg_dump` через `docker compose exec`, сжимает gzip;
- имя: `mega-trainer-YYYYMMDD-HHMMSS.sql.gz`;
- удаляет файлы старше `RETENTION_DAYS` (по умолчанию 14);
- при ошибке удаляет неполный дамп и завершается с кодом ≠ 0.

Восстановление:

```bash
gunzip -c backups/mega-trainer-20260101-030000.sql.gz \
  | docker compose -f docker/docker-compose.yml --env-file .env \
      exec -T postgres sh -c 'psql -U "$POSTGRES_USER" "$POSTGRES_DB"'
```

Рекомендуется запускать по cron/systemd-timer ежедневно и хранить копии вне хоста.

## Health и метрики

```bash
curl -s localhost:9090/healthz    # процесс жив
curl -s localhost:9090/readyz     # БД и Redis доступны
curl -s localhost:9090/metrics    # Prometheus
```

Docker-образ имеет `HEALTHCHECK` на `/healthz`. Порт задаётся в
`monitoring.port` (по умолчанию 9090).

## Логи

- stdout в JSON или console (`logging.format`), опционально файл
  (`logging.output: file` + `file_path`).
- Ротация в systemd — через logrotate (пример в старом README удалён; актуальный
  вариант — `journalctl` или logrotate на `file_path`).
- Уровень: `LOG_LEVEL=debug|info|warn|error`.

## Обновление

1. `git pull` / новый образ.
2. `make migrate-up` (или перезапуск compose — сервис `migrate` сделает сам).
3. Перезапустить `bot` и `cron`.
4. Проверить `/readyz` и `/start` в Telegram.

Откат: вернуть предыдущий бинарник/образ; схемы откатываются `migrate down`
только если изменение схемы было последней миграцией.

## Типовые проблемы

| Симптом | Причина | Решение |
|---|---|---|
| `authorize telegram bot: ... connection refused` | Telegram заблокирован | задать `TELEGRAM_PROXY_URL` |
| Бот запущен, но не отвечает | активный webhook (409) | удаляется автоматически при старте; проверить, что запущен новый бинарник |
| `invalid configuration: ...` | не заданы env | заполнить `.env` |
| `ping postgres: connection refused` | БД не поднята / неверный host | `docker compose ps`, проверить `POSTGRES_HOST` |
| `/chart` пишет «минимум два измерения» | мало данных | записать вес через `/measure` |
| В Docker не работает прокси `127.0.0.1` | localhost контейнера | `host.docker.internal` или запуск на хосте |
