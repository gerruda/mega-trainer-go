# Локальный запуск и проверка бота в Telegram

Пошаговая инструкция: поднять инфраструктуру, запустить бота из исходников,
проверить все команды в реальном Telegram и убедиться, что данные пишутся в БД.

Время на полный прогон: ~15 минут (из них основное — первый `go mod download`).

---

## 1. Что нужно заранее

| Компонент | Проверка | Комментарий |
|---|---|---|
| Go 1.27+ | `go version` | версия из `toolchain` в `go.mod` |
| Docker + Compose v2 | `docker compose version` | для PostgreSQL и Redis |
| Токен бота | есть от [@BotFather](https://t.me/BotFather) | должен быть живым |
| HTTP(S)-прокси до Telegram | см. шаг 3 | обязателен, если `api.telegram.org` заблокирован |
| Свободные порты | `5432`, `6379`, `9090` | PostgreSQL, Redis, health/metrics |

Проверить, что порты не заняты:

```bash
ss -ltn | grep -E ':5432|:6379|:9090' || echo "порты свободны"
```

---

## 2. Настроить `.env`

```bash
cp .env.example .env   # если файла ещё нет
```

Заполнить значения:

| Переменная | Пример | Обязательна |
|---|---|---|
| `BOT_TOKEN` | `1234567890:AA...` от @BotFather | да |
| `POSTGRES_HOST` | `localhost` | да |
| `POSTGRES_USER` | `mega_trainer` | да |
| `POSTGRES_PASSWORD` | пароль БД | да |
| `POSTGRES_DB` | `mega_trainer` | да |
| `REDIS_ADDR` | `localhost:6379` | да |
| `TELEGRAM_PROXY_URL` | `http://127.0.0.1:12334` | да, если Telegram заблокирован |
| `LOG_LEVEL` | `debug` для отладки | нет |

Рекомендация: на время проверки поставить `LOG_LEVEL=debug` — в логах будет видно
больше деталей.

> `TELEGRAM_PROXY_URL` должен быть доступен **с той машины, где запускается бот**.
> Если прокси на этом же хосте — `http://127.0.0.1:<порт>`. Если на роутере/другой
> машине — её IP (например, `http://10.0.1.66:12334`).

---

## 3. Проверить прокси и токен до запуска бота

```bash
set -a; . ./.env; set +a

curl -s -o /dev/null -w 'HTTP %{http_code}\n' \
  -x "$TELEGRAM_PROXY_URL" \
  "https://api.telegram.org/bot${BOT_TOKEN}/getMe"
```

Ожидается `HTTP 200`. Посмотреть ответ (убедиться, что это ваш бот):

```bash
curl -s -x "$TELEGRAM_PROXY_URL" "https://api.telegram.org/bot${BOT_TOKEN}/getMe" \
  | cut -c1-200; echo
```

Если здесь ошибка (`000`, `connection refused`, `407`) — сначала почините прокси,
бот без него не стартует.

---

## 4. Поднять PostgreSQL и Redis

```bash
make docker-up-dev
```

Проверить, что оба контейнера поднялись и стали healthy:

```bash
docker compose -f docker/docker-compose.dev.yml --env-file .env ps
```

Ожидается: `mega-trainer-postgres` и `mega-trainer-redis` в состоянии `healthy`.
Если порт занят — остановите локальный PostgreSQL/Redis или поменяйте порты в
`docker/docker-compose.dev.yml`.

---

## 5. Собрать бинарники

```bash
make build
```

Появятся `bin/bot`, `bin/cron`, `bin/migrate`. Первая сборка скачивает зависимости.

---

## 6. Применить миграции

```bash
make migrate-status   # 0001_init и 0002_measurements должны быть PENDING/APPLIED
make migrate-up       # применить
```

Повторный `make migrate-up` должен написать «неприменённых миграций нет» —
это проверка идемпотентности.

---

## 7. Запустить бота (терминал №1)

```bash
make run
```

`make run` сам соберёт бинарники, применит миграции и запустит бота.
Ожидаемые строки в логе:

```text
INF [cmd/bot:main] запуск бота version=... environment=development
INF [monitoring:Start] сервер метрик запущен addr=:9090
INF [app:Run] запросы к Telegram идут через прокси proxy=...
INF [app:Run] бот запущен environment=development username=<ваш_бот> workers=8
```

Признаки проблем:

| В логе | Причина |
|---|---|
| `authorize telegram bot ... connection refused` | прокси не работает |
| `invalid configuration: ...` | не заполнен `.env` |
| `ping postgres: connection refused` | не поднята инфраструктура |
| `ping redis: connection refused` | не поднят Redis |

> **Важно:** с одним токеном может работать только один экземпляр бота.
> Остановите прод/старый процесс, иначе updates будет забирать он, а локальный
> будет «молчать».

---

## 8. Проверить health и метрики (терминал №2)

```bash
curl -s localhost:9090/healthz; echo
curl -s localhost:9090/readyz; echo
curl -s localhost:9090/metrics | grep -E 'mega_trainer_telegram_updates'
```

Ожидается:

```text
{"status":"ok"}
{"status":"ready"}
mega_trainer_telegram_updates_processed_total 0
```

`/readyz` проверяет реальные подключения к PostgreSQL и Redis — это лучший
индикатор «всё поднялось».

---

## 9. Проверить сценарии в Telegram

Откройте своего бота и пройдите чек-лист. После каждого шага смотрите лог в
терминале №1 — ошибки будут с префиксом `[...]` и русским описанием.

| # | Действие | Ожидаемый результат |
|---|---|---|
| 1 | `/start` | приветствие и список команд |
| 2 | `/water` → `70,5` → «Средняя» | **2.54 л** в день |
| 3 | `/water` → `abc` | «Введите вес числом…», диалог остаётся на шаге веса |
| 4 | `/tdee` → Мужской → `30` → `80` → `180` → Умеренная → Поддержание | BMR **1780**, TDEE **2759** ккал |
| 5 | `/onerepmax` → «Бжицки» → `100` → `5` | **112.5 кг** |
| 6 | `/bodyfat` → Мужской → `30` → `80` → `180` → `40` → `90` | **~18.4 %** и «Измерение сохранено» |
| 7 | `/measure` → `70`, затем `/measure` → `70.5` | «Записано: …» |
| 8 | `/chart` | фото графика веса |
| 9 | `/history` | два измерения, новые сверху |
| 10 | `/export` | файл `measurements.csv` |
| 11 | `/cancel` в середине диалога | «Диалог отменён» |
| 12 | `/chart` у нового пользователя | «нужно минимум два измерения» |

Полезно проверить перезапуск: начать `/water`, остановить бота (Ctrl+C),
запустить снова и продолжить с того же шага — состояние живёт в Redis.

---

## 10. Проверить данные в PostgreSQL

В терминале №2 (переменные из `.env` подхватываются контейнером):

```bash
docker exec -it mega-trainer-postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT id, telegram_id, username, created_at FROM users ORDER BY id DESC LIMIT 5;"
```

Проверить измерения (появятся после `/measure` и `/bodyfat`):

```bash
docker exec -it mega-trainer-postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT id, user_id, weight_kg, body_fat_pct, measured_at FROM measurements ORDER BY measured_at DESC LIMIT 5;"
```

Токен и пароль в команды не подставляются — они берутся из окружения контейнера.

---

## 11. Проверить напоминания (терминал №3)

Планировщик в обычном режиме отправляет первое напоминание только через
`reminders.interval_hours` (по умолчанию 24 ч). Для быстрой проверки — разовый
режим:

```bash
make remind-once
```

Ожидаемо: лог `[cron:run] разовая рассылка напоминаний` и
`[cron:Once] рассылка напоминаний завершена sent=1 failed=0 users=1`, а в
Telegram приходит «⚖️ Напоминание: запишите текущий вес — /measure».

Длительный режим (как в проде): `make run-cron` (останавливается Ctrl+C).

---

## 12. Прогнать автотесты на живой инфраструктуре

Пока PostgreSQL и Redis подняты (терминалы бота/крона можно оставить):

```bash
make test          # unit, быстро
make test-infra    # integration + e2e, параметры берутся из .env
```

`make test-infra` создаёт временные схемы в БД и удаляет их после прогона —
данные разработчика не затрагиваются.

---

## 13. Бэкап (опционально)

```bash
# для dev-инфраструктуры нужен именно dev-compose:
COMPOSE_FILE=docker/docker-compose.dev.yml make backup

ls -lh backups/           # mega-trainer-<дата>.sql.gz
```

Для полного docker-стека (`make docker-up`) достаточно `make backup`.

---

## 14. Остановка

1. Бот — `Ctrl+C`; в логе должно быть
   `[app:Run] бот остановлен (контекст завершён)` — это проверка graceful shutdown.
2. Крон — `Ctrl+C`.
3. Инфраструктура:

```bash
make docker-down-dev          # остановить, данные сохранятся
make docker-down-dev && docker volume rm mega-trainer-dev_postgres_data   # снести данные
```

---

## 15. Типовые проблемы

| Симптом | Что проверить |
|---|---|
| `authorize telegram bot: ... connection refused` | шаг 3: `curl -x "$TELEGRAM_PROXY_URL" ...`; адрес прокси доступен с этой машины |
| Бот запущен, но не отвечает | нет ли второго процесса с тем же токеном; нажмите `/start` ещё раз |
| `409 Conflict` или сообщения пропадают | тот же токен используется в другом экземпляре |
| `ping postgres: connection refused` | `make docker-up-dev`, проверьте `docker compose ps` |
| `bind: address already in use` для `:9090` | занят порт метрик: освободите его или поменяйте `monitoring.port` в `configs/config.yaml` |
| `make run` падает на миграциях | `make migrate-status`, логи PostgreSQL: `docker logs mega-trainer-postgres` |
| `/chart` пишет «минимум два измерения» | запишите вес дважды через `/measure` |
| В Docker не работает прокси `127.0.0.1` | внутри контейнера это сам контейнер: используйте `host.docker.internal` или запускайте бота на хосте (как в шаге 7) |
| `make backup` не находит сервис | укажите `COMPOSE_FILE=docker/docker-compose.dev.yml` или поднимите полный стек (`make docker-up`) |

---

## 16. Чек-лист «всё готово»

- [ ] `curl` к `getMe` через прокси возвращает 200.
- [ ] `docker compose ps` — postgres и redis healthy.
- [ ] `make migrate-status` — обе миграции APPLIED.
- [ ] Бот стартует, в логе 4 строки из шага 7.
- [ ] `/readyz` возвращает `{"status":"ready"}`.
- [ ] Все 12 пунктов шага 9 отработали.
- [ ] В `users` есть запись, в `measurements` — строки после `/measure`.
- [ ] `make remind-once` отправил напоминание.
- [ ] `make test-infra` зелёный.
- [ ] Ctrl+C у бота → строка о graceful shutdown.
