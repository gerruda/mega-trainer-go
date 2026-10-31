# Mega Trainer Bot

Telegram-бот на Go: персональный фитнес-трекер. Считает прикладные показатели
тренировок и питания, хранит пользователей в PostgreSQL, состояние диалогов — в Redis.

[![CI](https://github.com/gerruda/mega-trainer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/gerruda/mega-trainer-go/actions/workflows/ci.yml)

## Возможности

| Команда | Что делает |
|---|---|
| `/water` | норма воды с поправкой на активность |
| `/onerepmax` | одноповторный максимум по формулам Эпли, Бжицки и Ломбарди |
| `/bodyfat` | процент жира по формуле ВМС США (Navy) |
| `/tdee` | BMR/TDEE по Mifflin–St Jeor и целевая калорийность |
| `/cancel` | прервать текущий диалог |
| `/measure` | записать вес (дневник измерений) |
| `/chart` | график динамики веса (PNG) |
| `/history` | последние 10 измерений |
| `/export` | выгрузка измерений в CSV |

Диалоги пошаговые: состояние шага и собранные ответы хранятся в Redis с TTL 24 часа,
каждый пользователь обрабатывается последовательно (без гонок и «прыжков» между шагами).

## Технологии

- **Go 1.24**, стандартная библиотека
- **PostgreSQL 16** + `sqlx` — пользователи и (далее) измерения
- **Redis 7** — FSM-состояние диалогов
- **telegram-bot-api/v5** — Telegram Bot API
- **zerolog** — структурированное логирование
- **Prometheus** — метрики и health/readiness-пробы
- **go-chart** — рендеринг графиков в PNG
- **HTTP(S)-прокси** — для сетей, где `api.telegram.org` заблокирован (например, в РФ)

## Архитектура

Зависимости направлены внутрь: `delivery → service → domain`, `storage → domain`.
`domain` не знает ни об одной инфраструктуре.

```
cmd/bot            composition root: конфиг → БД/Redis → сервисы → доставка
cmd/cron           планировщик напоминаний (отдельный процесс)
cmd/migrate        применение SQL-миграций (embed.FS, advisory lock)
internal/domain    сущности и интерфейсы репозиториев
internal/service   формулы, FSM-хелперы, пользователи, измерения
internal/storage   реализации репозиториев (postgres, redis)
internal/delivery  Telegram-транспорт и диалоги
internal/telegramapi клиент Bot API, HTTP(S)-прокси, удаление webhook
internal/cron      фоновые задачи (напоминания)
internal/monitoring healthz/readyz/metrics HTTP-сервер
migrations         SQL-схема (единственный источник истины)
```

## Быстрый старт

### Локально (бот на хосте, инфраструктура в Docker)

```bash
cp .env.example .env          # впишите BOT_TOKEN от @BotFather
make docker-up-dev            # PostgreSQL + Redis
make run                      # миграции + бот
```

### Полный стек в Docker

```bash
cp .env.example .env          # впишите BOT_TOKEN
make docker-up                # postgres → migrate → bot
```

Подробная пошаговая инструкция с чек-листом проверки в реальном Telegram —
[`docs/LOCAL_RUN.md`](docs/LOCAL_RUN.md).

Проверка состояния:

```bash
curl -s http://localhost:9090/healthz
curl -s http://localhost:9090/readyz
curl -s http://localhost:9090/metrics | head
```

## Конфигурация

Настройки лежат в [`configs/config.yaml`](configs/config.yaml) и читаются с подстановкой
переменных окружения. Секреты в репозиторий не попадают: реальные значения — в `.env`
(см. [`.env.example`](.env.example)).

| Переменная | Назначение | Обязательна |
|---|---|---|
| `BOT_TOKEN` | токен Telegram-бота | да |
| `POSTGRES_HOST` / `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | подключение к PostgreSQL | да |
| `TELEGRAM_PROXY_URL` | HTTP(S)-прокси для Telegram, например `http://127.0.0.1:12334` | нет |
| `REDIS_ADDR` | адрес Redis, например `localhost:6379` | да |
| `REDIS_PASSWORD` | пароль Redis | нет |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | нет |
| `CONFIG_PATH` | путь к YAML-конфигу (по умолчанию `configs/config.yaml`) | нет |

При старте конфиг валидируется: процесс завершается с понятным сообщением обо всех
пропущенных обязательных параметрах сразу.

### Прокси для Telegram

В сетях без прямого доступа к `api.telegram.org` укажите HTTP(S)-прокси:

```bash
# .env
TELEGRAM_PROXY_URL=http://127.0.0.1:12334
```

Поддерживаются только схемы `http://` и `https://` (этого достаточно: Bot API — это
обычный HTTPS API). При старте в логах будет строка `telegram requests go through a proxy`
с хостом прокси; креденшелы в лог не попадают. Если `getMe` падает с `connection refused`,
в ошибке будет подсказка про `TELEGRAM_PROXY_URL`.

Если бот запущен в Docker, `127.0.0.1` внутри контейнера — это сам контейнер: используйте
`host.docker.internal` (Docker Desktop) или запускайте бота на хосте (`make run`).

## Миграции

Схема применяется отдельной командой (в Docker — сервисом `migrate`):

```bash
make migrate-up       # применить все миграции
make migrate-status   # статус
make migrate-down     # откатить последнюю
```

## Напоминания и бэкапы

```bash
make run-cron    # планировщик напоминаний (интервал — reminders.interval_hours)
make backup      # gzip-дамп PostgreSQL в ./backups + ротация
```

Напоминания отключаются в `configs/config.yaml` (`reminders.enabled: false`).
Подробности — в [`docs/OPERATIONS.md`](docs/OPERATIONS.md).

## Разработка

```bash
make build            # бинарники в ./bin
make test             # unit-тесты с -race
make test-integration # репозитории и миграции на живом PostgreSQL
make test-e2e         # сквозной сценарий бота против fake Telegram API
make test-infra       # integration + e2e, параметры берутся из .env
make test-all         # все уровни
make cover            # покрытие
make lint             # gofmt + go vet + golangci-lint (v2, из PATH или ./.tools)
make fmt              # форматирование
```

| Уровень | Что проверяет | Требования |
|---|---|---|
| unit | формулы, FSM, конфиг, cron, прокси | нет |
| integration | репозитории и миграции, изоляция схем | `TEST_POSTGRES_DSN` |
| e2e | весь пайплайн: fake Bot API → бот → PostgreSQL/Redis → ответы | `TEST_POSTGRES_*`, `TEST_REDIS_ADDR` |

E2E-тесты поднимают фейковый Telegram Bot API (`httptest`), поэтому не требуют токена
и доступа к `api.telegram.org`. Каждый прогон создаёт временную схему в PostgreSQL и
удаляет её после теста — данные разработчика не затрагиваются.

CI прогоняет `gofmt`, `go build`, `go vet`, `go test -race`, `golangci-lint`,
а также integration- и e2e-джобы с сервисами PostgreSQL и Redis.

Docker-образ собирается из `golang:${GO_VERSION}-alpine`, где `GO_VERSION` — версия из
`toolchain` в `go.mod` (build-arg в `docker/Dockerfile`, держите их синхронно).

## Статус и дорожная карта

- [x] **Фаза 0** — проект собирается, тесты и CI, конфиг с валидацией, Makefile, Docker
- [x] **Фаза 1** — единый домен и формулы, безопасные миграции, FSM без потери данных,
      graceful shutdown, health/metrics, юнит-тесты
- [x] **Фаза 2** — дневник измерений (`/measure`), `/chart`, `/history`, `/export`,
      автосохранение после `/bodyfat`, напоминания (`cmd/cron`), бэкапы
- [x] **Фаза 3** — архитектура и ADR, тест-план, лицензия, CONTRIBUTING, CI,
      golangci-lint, интеграционные и e2e-тесты; осталось: релизный тег

## Документация

| Документ | О чём |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | слои, потоки данных, схема БД, принятые решения |
| [`docs/LOCAL_RUN.md`](docs/LOCAL_RUN.md) | пошаговый локальный запуск и проверка в Telegram |
| [`docs/OPERATIONS.md`](docs/OPERATIONS.md) | запуск, миграции, напоминания, бэкапы, troubleshooting |
| [`docs/TEST_PLAN.md`](docs/TEST_PLAN.md) | план тестирования и матрица кейсов |
| [`docs/AUDIT_AND_PLAN.md`](docs/AUDIT_AND_PLAN.md) | аудит, приоритеты, дорожная карта |
| [`docs/adr/`](docs/adr/0001-single-calculator.md) | Architecture Decision Records |
| [`AGENTS.md`](AGENTS.md) | правила разработки и работы ИИ-агентов |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | как контрибьютить |

## Лицензия

[MIT](LICENSE).
