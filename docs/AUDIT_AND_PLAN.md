# Аудит репозитория и план приведения в порядок

**Проект:** `mega-trainer-go` — Telegram-бот (фитнес-трекер) на Go + PostgreSQL + Redis + графики.
**Дата аудита:** исходное состояние на коммите `8e05523` («обновил реализацию»), ветка `master`.
**Роль владельца:** QA → middle Go developer. **Аудитория репозитория:** HR, тимлиды, техлиды, ИИ-ревьюеры.
**Статус:** Фазы 0 и 1 выполнены, Фаза 2 закрыта в основной части; добавлена поддержка
HTTP(S)-прокси для Telegram. Отчёты и проверки — в разделах B.7 и B.8. Часть A описывает
состояние **до** исправлений и сохранена как история решения.

Документ состоит из двух частей:

- **Часть A — аудит:** что именно не так, с доказательствами (файл:строка) и приоритетом.
- **Часть B — план:** фазы, задачи, оценки, критерии готовности, предложения по продукту и процессу.

---

# Часть A. Аудит

## A.0. Как воспроизводилось

```bash
git clone https://github.com/gerruda/mega-trainer-go && cd mega-trainer-go
gofmt -l .          # main.go — единственный неотформатированный файл
go build ./...      # FAIL: cmd/cron, cmd/migrate — "function main is undeclared"
go vet ./...        # FAIL: 6 замечаний (Msgf без форматных директив)
go test ./...       # 0 тестов во всех пакетах; internal/service не проходит из-за vet
```

Окружение: Go 1.24.4, зависимости скачиваются (`go mod download`), Docker в CI не настроен.

## A.1. Состояние в цифрах

| Метрика | Значение |
|---|---|
| Go-файлов / строк | 33 / 3 176 |
| Файлов в git | 48 |
| Пустых файлов (0 байт) | 5 |
| Файлов-заглушек (`package x` без кода) | 5 |
| Тестов | **0** (покрытие 0 %) |
| CI/CD | отсутствует |
| `go build ./...` | **FAIL** |
| `go vet ./...` | **FAIL** |
| Коммитов в истории | 5, из них HEAD = 2 967 вставок одним коммитом |
| Ветки / теги / PR | нет |
| `README.md` | начинается с примера logrotate, не описывает проект |
| Docker | образ бота **не собирается**, cron-образ невозможен (пустой Dockerfile) |
| Целевая фича «графики» | код есть, **не вызывается нигде** |
| Целевая фича «БД» | подключение создаётся и сразу закрывается, репозитории не используются |

## A.2. Проблемы P0 — «репозиторий не собирается / не запускается»

| # | Проблема | Доказательство | Влияние |
|---|---|---|---|
| P0-1 | В корне лежит шаблон GoLand «Hello, gopher» — второй `package main` в модуле | `main.go:1-21` | `go build ./...` собирает мусор; первый файл, который видит HR |
| P0-2 | `cmd/cron` и `cmd/migrate`: `package main` без `func main()` | `cmd/cron/main.go:1`, `cmd/migrate/main.go:1` | **сборка всех пакетов падает** |
| P0-3 | Файлы-заглушки без кода | `internal/cron/jobs.go:1`, `internal/service/metrics_service.go:1`, `pkg/monitoring/health.go:1` | мёртвый код, создаёт ложное впечатление «реализовано» |
| P0-4 | Пустые файлы (0 байт) в git | `docker/Dockerfile.cron`, `scripts/backup.sh`, `scripts/deploy.sh`, `deployments/systemd/cron.service`, `deployments/timeweb/setup.md` | compose-сервис `cron` не собирается; деплой/бэкап не существуют |
| P0-5 | Нет конфига, который требует Dockerfile | `docker/Dockerfile.bot:16` копирует `configs/production.yaml`; каталога `configs/` нет, `.gitignore:2` его игнорирует | `docker build` падает на `COPY` |
| P0-6 | Приложение падает без `.env` | `internal/config/config.go:77-80` — `godotenv.Load(".env")` возвращает ошибку наружу | не стартует в Docker/CI; README при этом требует файл `env.env` (`README.md:13-18`) — расхождение |
| P0-7 | `docker-compose.yml` монтирует несуществующий путь и не передаёт токен | `docker/docker-compose.yml:14` (`./migrations`), `:35-43` (bot без `BOT_TOKEN`), `env_file` отсутствует | прод-развёртывание нерабочее в принципе |
| P0-8 | `defer pg.Close()` внутри конструктора | `internal/bot/bot.go:45` | соединение с БД закрывается сразу после `NewBot`; любой запрос к БД вернёт `sql: database is closed` |
| P0-9 | `go vet` не проходит, тестов нет | см. A.0 | нет ни одного автоматического подтверждения работоспособности |
| P0-10 | Два конфликтующих механизма миграций | `migrations/001_create_users.sql` vs `internal/storage/postgres/migration.go:80-197` | схемы `users` разные (`BIGINT` без sequence vs `SERIAL`); кто выполнится первым — второй упадёт. Файл `001` содержит секцию `-- down` в том же файле (`:18-20`), и любой исполнитель (psql, docker-entrypoint) выполнит и `DROP TABLE` |
| P0-11 | БД-фича не подключена к боту | `internal/bot/bot.go:25-72`: `userRepo`, `user_service`, `migration manager` не создаются; `cmd/bot/main.go:77` — закомментировано | «бот с поддержкой БД» на деле работает без БД |
| P0-12 | Функция «графики» не вызывается | `pkg/plot/plot.go:10` — `RenderLineChart` не используется ни в одном файле; измерений тела нет ни в одном репозитории | ключевая заявленная возможность отсутствует |

## A.3. Проблемы P1 — архитектура и корректность

### Архитектура / дублирование

| # | Проблема | Доказательство |
|---|---|---|
| P1-1 | Две параллельные модели `User` | `internal/domain/user.go:3` (поля `ChatID`, `CreatedAt string`) и `internal/storage/postgres/repo.go:98` (полная модель с `db`-тегами) |
| P1-2 | Два конкурирующих интерфейса `UserRepository` | `internal/domain/repository.go:5` (`GetByChatID`) и `internal/storage/postgres/repo.go:49` (`GetByTelegramID`). Доменный интерфейс не реализуется никем |
| P1-3 | Два калькулятора с разными формулами | `internal/service/calculator.go:14-86` (Mifflin–St Jeor) и `pkg/calculator/calculator.go:30-38` (Harris–Benedict). Результаты TDEE расходятся |
| P1-4 | Перевёрнутые зависимости: `pkg` импортирует `internal` | `pkg/logger/logger.go:10`, `pkg/database/postgres.go:10`, `pkg/telegram/types.go:7` |
| P1-5 | Сервис зависит от конкретной реализации хранилища | `internal/service/user_service.go:7,13,19` — `postgres.User`, `postgres.UserRepository` вместо доменных типов/интерфейсов |
| P1-6 | Двойной транспортный слой | `internal/service/telegram_service.go` (готовый `TelegramService`) не используется: хендлер шлёт сообщения напрямую `h.bot.Send` — 26 мест |
| P1-7 | Два набора метрик с разными именами и дублирующимися счётчиками | `internal/bot/metrics.go:9-59` (не используется вовсе) и `pkg/monitoring/metrics.go:13-76` (используется только конструктор) |
| P1-8 | `pkg/monitoring/health.go` пустой — health-check отсутствует, хотя `Config` и `docker-compose` его подразумевают | `internal/config/config.go:61-66` (`HealthCheckPort`) |

### Гонки и паники (runtime)

| # | Проблема | Доказательство |
|---|---|---|
| P1-9 | Гонка данных: `messageHistory` — обычная map, а каждый update обрабатывается в новой горутине | `internal/delivery/telegram/handler.go:15`, `internal/bot/bot.go:83-89` |
| P1-10 | Неограниченная горутина на каждый update + отсутствие ограничения параллелизма | там же; при шторме сообщений бот исчерпает ресурсы |
| P1-11 | Неограниченный рост `messageHistory` (утечка памяти) | `internal/delivery/telegram/helpers.go:62-64` — запись без TTL/лимита |
| P1-12 | Потеря данных FSM: сохранённая формула затирается | `internal/delivery/telegram/onerepmax.go:34` сохраняет `formula`, `:42` вызывает `SetUserState(..., 2, nil)` — `data` = nil → в расчёте всегда `epley` |
| P1-13 | Возможный nil-deref | `internal/delivery/telegram/callbacks.go:97-108` — `currentState` может быть `nil`; `handler.go:86-89` — `callback.Message` может быть `nil`; `pkg/plot/plot.go:11-13` возвращает `(nil, nil)` |
| P1-14 | Ошибки отправки игнорируются, затем используется результат | `helpers.go:21`, `helpers.go:57`, `onerepmax.go:26`, `water.go:23`, `callbacks.go:74` и др. При сбое `Send` возвращается `Message{}` с `MessageID=0` — «удаление сообщения 0» и т. п. |
| P1-15 | `log.Fatal`/`panic` в библиотечном коде | `internal/bot/bot.go:43,55`; `pkg/logger/logger.go:50,55` — процесс умирает без шанса корректно закрыть ресурсы |
| P1-16 | Сравнение ошибок без `errors.Is` | `internal/service/user_service.go:38`, `internal/storage/postgres/user_repo.go:142` |
| P1-17 | Graceful shutdown не работает | `cmd/bot/main.go:35` подписывается только на `os.Interrupt` (Docker шлёт `SIGTERM`); `internal/bot/bot.go:83` — `for update := range updates` не реагирует на `ctx.Done()`, воркеры не дожидаются |
| P1-18 | Внешние вызовы без контекста/таймаутов | `internal/storage/postgres/migration.go:39,48,68` (`db.Exec/Query` без `Context`), `user_repo.go` в целом ок, а Telegram — без таймаута |
| P1-19 | Markdown без экранирования | `internal/service/telegram_service.go:32`, `internal/delivery/telegram/*` — `ParseMode=Markdown` + динамический текст; пользовательские строки с `_`/`*` вызовут ошибку Telegram API |
| P1-20 | `config.expandEnvVars` — мёртвый код | `internal/config/config.go:96,101-114`; при этом нет валидации обязательных полей и defaults |
| P1-21 | Миграции небезопасны | `migration.go:225-248` — нет `defer tx.Rollback()`, не проверяется `rows.Err()` (`:55-62`), нет advisory lock (два процесса применят миграции одновременно), `fmt.Printf` вместо логгера |
| P1-22 | Дублирование подключения к БД | `pkg/database/postgres.go:17` (`*sql.DB`) и `migration.go:332` (`*sqlx.DB`, пул 25/5 захардкожен) |

## A.4. Проблемы P2 — качество, документация, DX, «витрина»

| # | Проблема | Доказательство |
|---|---|---|
| P2-1 | README не описывает продукт: начинается с logrotate, нет архитектуры, скриншотов, быстрого старта | `README.md:1-36` |
| P2-2 | Нет `.github/` (CI, шаблоны PR/issue, CODEOWNERS), лицензии, `Makefile`, `.dockerignore`, `golangci-lint` конфига | отсутствуют |
| P2-3 | Нет тестовой инфраструктуры (моки, фикстуры, `testcontainers`/`miniredis`) | `find . -name '*_test.go'` — пусто |
| P2-4 | `.gitignore` не покрывает реальные артефакты | `.gitignore:1-3`: `/*.env` не ловит `docker/.env`, `configs/*.yaml` игнорируется целиком (нельзя закоммитить example), нет `*.out`, `coverage`, `.gocache` |
| P2-5 | Docker: root-пользователь, нет `HEALTHCHECK`, нет `-trimpath -ldflags`, нет кэширования слоёв `go mod download` | `docker/Dockerfile.bot:1-16` |
| P2-6 | systemd-юнит ссылается на другой каталог/имя проекта и грузит весь `/etc/environment` | `deployments/systemd/bot.service:5,7,8` (`/opt/megan-telegram-bot`, `EnvironmentFile=/etc/environment`) |
| P2-7 | `docker-compose.yml`: устаревший `version: '3.8'`, `depends_on` без healthcheck (бот стартует раньше БД), пароли в env, нет лимитов | `docker/docker-compose.yml:1,30-33` |
| P2-8 | Инициализационный скрипт БД выдаёт боту `SUPERUSER` | `docker/postgres/init/01-create-user.sh:33` |
| P2-9 | Логи: PII (username) в логах, `Msgf` со структурными полями (это же и ошибка `go vet`) | `internal/service/user_service.go:51-54`, `telegram_service.go:41-103` |
| P2-10 | Метрики объявлены, но нигде не инкрементируются; `promauto` регистрирует глобальные коллекторы при каждом вызове `NewMetrics` | `pkg/monitoring/metrics.go:26-62` |
| P2-11 | История git неудобна для ревью: один коммит на ~3 000 строк, нет атомарности | `git show --stat 8e05523` |
| P2-12 | `deployments/timeweb/setup.md` пустой — инструкция по деплою не написана | 0 байт |
| P2-13 | `telegram.NewHandler` принимает `*tgbotapi.BotAPI` — тестировать без сети невозможно | `internal/delivery/telegram/handler.go:11-29` |
| P2-14 | Нет `context` в `StateService.ClearUserState`-цепочках ошибок, но есть `log.Printf` в сервисе (двойное логирование) | `internal/service/state.go:20-36` |

## A.5. Что сделано хорошо (важно сохранить)

Чтобы не «снести всё»: в проекте уже есть **правильный каркас**, его нужно довести, а не переписать.

- Слои `internal/{delivery,service,storage,domain}` заведены и в целом логичны.
- Есть `context` в репозиториях, обёртка ошибок `%w`, `sqlx`, пул соединений.
- FSM на Redis с TTL — верное решение для бота.
- `go-chart` + рендер в `[]byte` — правильный подход к графикам в Telegram.
- Прометеус-метрики и zerolog выбраны адекватно.
- `docker-compose.dev.yml` с healthcheck и отдельным init-скриптом — хорошая основа.

---

# Часть B. План

## B.1. Принципы

1. **Сначала «зелёный клон»**, потом фичи. Пока `go build ./...` не проходит — ничего нового не пишем.
2. **Маленькие коммиты.** Каждый пункт плана — отдельный PR/коммит с внятным сообщением.
3. **Один источник правды** для моделей, формул и миграций.
4. **Удалять, а не комментировать.** Мёртвый код — отдельным коммитом «chore: remove dead code».
5. **Каждое изменение проверяемо**: тест, команда или воспроизводимый сценарий в PR.

## B.2. Дорожная карта

### Фаза 0 — «Здоровый клон» (P0, ~4–6 ч)
**Цель:** `git clone && make build && make vet && make test` работает без `.env`, без Docker.
**Критерий готовности:** зелёные quality gates в CI на пустом раннере.

| ID | Задача | Файлы | Приоритет |
|---|---|---|---|
| 0.1 | Удалить корневой `main.go` (шаблон GoLand) | `main.go` | P0 |
| 0.2 | Реализовать `cmd/migrate` (флаги `up|down|status`) и `cmd/cron` (пока — заглушка с `func main` + TODO-задачей) **или** удалить оба пакета до Фазы 2 | `cmd/migrate`, `cmd/cron` | P0 |
| 0.3 | Удалить пустые/заглушечные файлы; нереализованное вынести в явный backlog | `internal/cron/jobs.go`, `internal/service/metrics_service.go`, `pkg/monitoring/health.go`, `docker/Dockerfile.cron`, `scripts/*`, `deployments/*` | P0 |
| 0.4 | Добавить `configs/config.example.yaml`, `.env.example`, валидацию конфига и defaults; убрать жёсткое падение без `.env` | `internal/config/config.go`, `configs/` | P0 |
| 0.5 | `.gitignore`: разрешить `configs/*.example.yaml`, добавить `*.out`, `coverage*`, `.idea/`, `.gocache/`; `.dockerignore` | `.gitignore`, `.dockerignore` | P0 |
| 0.6 | `Makefile`: `build`, `run`, `test`, `test-race`, `lint`, `docker-build`, `migrate-up` | `Makefile` (новый) | P1 |
| 0.7 | CI: GitHub Actions — `gofmt -l`, `go build`, `go vet`, `go test -race`, `golangci-lint`, кэш модулей | `.github/workflows/ci.yml`, `.golangci.yml` | P1 |
| 0.8 | Исправить `go vet` (6 замечаний): `Msgf` → `.Str()/.Int64()` | `internal/service/telegram_service.go`, `user_service.go` | P0 |
| 0.9 | Исправить Dockerfile/compose: multi-stage, non-root, `HEALTHCHECK`, передача env, `env_file`, убрать несуществующие COPY/volume | `docker/*` | P1 |

**Проверка:** `go build ./... && go vet ./... && go test ./... && gofmt -l .` — чисто; `docker build` проходит.

### Фаза 1 — «Одна архитектура, без гонок» (P1, ~12–16 ч)
**Цель:** один домен, один калькулятор, один механизм миграций, потокобезопасный FSM, корректный жизненный цикл приложения.

| ID | Задача | Файлы |
|---|---|---|
| 1.1 | Свести модели к `internal/domain`: `User`, `UserState`, `Calculation`, `BodyMeasurement`; удалить модели из `storage/postgres` и `pkg/telegram` | `internal/domain`, `internal/storage/postgres/repo.go`, `pkg/telegram/types.go` |
| 1.2 | Оставить один калькулятор (в `internal/service`) с тестами на формулы; удалить `pkg/calculator` | `internal/service/calculator.go`, `pkg/calculator` |
| 1.3 | Один механизм миграций: SQL-файлы `migrations/NNN_*.sql` + `embed.FS` + раннер с advisory lock; удалить `migration.go:80-329` | `migrations/`, `cmd/migrate`, `internal/storage/postgres/migrate.go` |
| 1.4 | Composition root: собрать в `cmd/bot/main.go` конфиг → БД → Redis → репозитории → сервисы → хендлер; убрать `defer pg.Close()` из конструктора | `cmd/bot/main.go`, `internal/bot/bot.go` |
| 1.5 | Потокобезопасность: `sync.Map`/мьютекс для `messageHistory` + TTL/лимит, или хранение истории в Redis; ограничить параллелизм воркеров (пул/семафор) | `internal/delivery/telegram`, `internal/bot/bot.go` |
| 1.6 | Ошибки и отмена: проверять `err` от `Send`, не использовать `sentMsg` при ошибке, `context` + таймауты, `signal.NotifyContext(SIGINT, SIGTERM)`, дождаться воркеров | `internal/delivery/telegram/*`, `internal/bot/bot.go`, `cmd/bot/main.go` |
| 1.7 | Убрать `log.Fatal`/`panic` из библиотечного кода; ошибки поднимать в `main` | `internal/bot/bot.go`, `pkg/logger/logger.go` |
| 1.8 | Экранирование Markdown (`html.EscapeString` для пользовательских данных или `ParseMode=HTML`) | `internal/delivery/telegram`, `internal/service/telegram_service.go` |
| 1.9 | Удалить мёртвый код: `expandEnvVars`, `StartWebhooks`, закомментированные блоки в `cmd/bot/main.go` (≈90 строк), дублирующий `TelegramService` | `internal/config`, `internal/bot`, `cmd/bot` |
| 1.10 | `go test -race ./...` + unit-тесты калькулятора и FSM (моки репозиториев) | `*_test.go` |

**Проверка:** бот запускается локально (`docker compose -f docker/docker-compose.dev.yml up -d` + `make run`), проходит сценарий `/start → /water → … → результат`; `go test -race ./...` зелёный.

### Фаза 2 — «Ядро продукта: данные и графики» (целевая фича, ~16–24 ч)
**Цель:** заработать заявленная связка «БД + графики по данным пользователя».

| ID | Задача | Результат |
|---|---|---|
| 2.1 | Схема БД: `users`, `body_measurements` (вес, % жира, обхваты, дата), `calculations` (тип, значение, metadata JSONB) | миграции + ER-диаграмма в `docs/` |
| 2.2 | Репозитории измерений: `SaveMeasurement`, `ListMeasurements(userID, from, to)` | `internal/storage/postgres/measurement_repo.go` + интеграционные тесты |
| 2.3 | Сохранение результатов из FSM (вода/1ПМ/%жира/TDEE) в `calculations` | история расчётов у пользователя |
| 2.4 | Команда `/chart`: график веса/% жира за период → PNG → `SendPhoto` | рабочая фича-витрина (скриншот в README) |
| 2.5 | `pkg/plot`: подписи осей, даты по X, обработка «нет данных», юнит-тесты рендера | график, который не стыдно показать |
| 2.6 | `/history` — последние N измерений, `/export` — CSV | польза для пользователя |
| 2.7 | Напоминания (cron + Telegram) — реализовать `cmd/cron` на базе `robfig/cron` | «бот с поддержкой ДБ» становится живым |
| 2.8 | Бэкапы: `scripts/backup.sh` (pg_dump + ротация) либо `internal/service/backup` | P0-4 закрыт |

**Проверка:** пройден e2e-сценарий в Docker: несколько измерений → `/chart` отдаёт картинку; данные выживают перезапуск контейнера.

### Фаза 3 — «Витрина и качество» (портфолио, ~12–16 ч)
**Цель:** репозиторий, который не стыдно отправить HR и на который ИИ-ревьюер не найдёт P0/P1.

| ID | Задача | Результат |
|---|---|---|
| 3.1 | README как витрина: проблема → решение → скриншоты бота и графика → архитектурная схема → быстрый старт за 3 команды → тесты/CI-бейджи → roadmap | главный файл для HR |
| 3.2 | `docs/ARCHITECTURE.md` + 2–3 ADR (почему FSM в Redis, почему go-chart, почему один калькулятор) | показывает инженерное мышление |
| 3.3 | `docs/TESTING.md` + `docs/TEST_PLAN.md`: матрица тест-кейсов, границы, негативные сценарии (используем QA-бэкграунд как преимущество) | сильный аргумент на собеседовании |
| 3.4 | Наблюдаемость: `/healthz`, `/readyz`, рабочие Prometheus-метрики (один набор), структурные логи с `request_id`, graceful shutdown | production-ready |
| 3.5 | Покрытие ≥ 60 % `internal/service` и `internal/domain`, бейдж покрытия | доверие ревьюера |
| 3.6 | `LICENSE` (MIT), шаблон PR, `CODEOWNERS`, dependabot | гигиена OSS |
| 3.7 | Прод-деплой: актуальный systemd-юнит (или `docker compose` + Traefik/nginx), `docs/DEPLOY.md` | закрывает P2-6/12 |
| 3.8 | Теги/релизы (`v0.1.0`), семантическое версионирование, `CHANGELOG.md` | зрелость проекта |

**Проверка:** внешний человек по README поднимает проект с нуля; ИИ-ревьюер не находит P0/P1.

## B.3. Оценка и последовательность

| Фаза | Оценка | Что можно показать после |
|---|---|---|
| 0 | 4–6 ч | «Собирается, тесты/CI зелёные» |
| 1 | 12–16 ч | «Архитектура без дублей и гонок, есть тесты» |
| 2 | 16–24 ч | «Работают БД и графики — заявленная фича» |
| 3 | 12–16 ч | «Готовое портфолио: README, CI, документация, релиз» |
| **Итого** | **44–62 ч** (≈ 1.5–2 недели в спокойном режиме) | |

Порядок обязателен: **0 → 1 → 2 → 3**. Фазы 2 и 3 нельзя начинать до зелёных quality gates.

## B.4. Предложения (сверх исправления ошибок)

**Продуктовые**

1. **Сделать графики ядром.** `/chart` — это то, чем проект отличается от «ещё одного калькулятора». Минимум 3 графика: вес, % жира, прогресс 1ПМ.
2. **Онбординг за 30 секунд:** `/start` сразу предлагает «записать измерение» или «посмотреть график», а не меню из 4 калькуляторов.
3. **Экспорт данных** (`/export` CSV) — дёшево в реализации, сильно повышает «взрослость» продукта.
4. **Напоминания** (cron) — второй процесс, который оправдывает наличие `cmd/cron` и PostgreSQL.

**Инженерные**

5. **Никакого `pkg/`** до появления внешнего API: всё в `internal/`, интерфейсы — в `domain`. Это снимает P1-4 целиком.
6. **Один раннер миграций** на `embed.FS` + advisory lock (или `pressly/goose`); SQL-файлы — единственный источник схемы.
7. **Ограничить параллелизм**: пул воркеров (`n = 8–16`) вместо горутины на update; FSM-состояние читать/писать атомарно (Redis `WATCH`/Lua или мьютекс на пользователя).
8. **Типизированные ошибки домена** (`domain.ErrNotFound`, `ErrValidation`) и маппинг в сообщения бота — вместо строк «Ошибка сохранения данных».
9. **Валидация входных данных** в сервисе, а не в хендлере: границы веса/роста/возраста, единицы измерения, локализация.
10. **Docker-образ**: `FROM scratch`/distroless, non-root, `-trimpath -ldflags="-s -w -X main.version=..."`, `HEALTHCHECK /healthz`.

**Для портфолио и процесса**

11. **Разбить текущую историю на осмысленные PR.** Даже если код уже написан, лучше провести его через 8–10 маленьких PR с Conventional Commits: ревьюер увидит процесс, а не «свалил 3 000 строк».
12. **Вести `docs/TEST_PLAN.md`** — ваш QA-опыт должен быть виден: граничные значения (reps=1, повтор 10, вес 0.1), негативные кейсы (текст вместо числа, `/chart` без данных), регрессия формул.
13. **Зафиксировать Definition of Done** в локальных инструкциях для агентов (вне репозитория) и следовать ему в каждом PR.
14. **Скриншоты в README**: диалог бота + пример графика. Для HR это сильнее любого описания.
15. **Roadmap-чекбоксы в README**: закрытые пункты плана демонстрируют прогресс и дисциплину.

## B.5. Риски и открытые вопросы

| Риск / вопрос | Комментарий |
|---|---|
| Нет Docker/СУБД в среде разработки | Фазы 0–1 проверяются без них; для 2.2/2.3 понадобится `testcontainers` или локальный compose |
| Смена схемы БД | Начальная «миграция» 001 могла уже примениться локально; нужен план миграции на новую схему (`users.id SERIAL`) и очистка dev-окружения |
| Telegram Bot API v5 | Библиотека `go-telegram-bot-api/v5` в поддержке, но v6 API-совместима; менять сейчас не нужно (не входит в цель) |
| Объём работ | 44–62 ч легко превратить в «перепишем всё». Держим строгую последовательность фаз и один PR за раз |
| Секреты | В истории секретов не найдено (проверено `git log -S`), но `.env`/`configs` должны оставаться вне git; при утечке токена — перевыпустить через `@BotFather` |
| Роль ИИ-ревьюеров | Ожидаемо придерутся к: пустым файлам, отсутствию тестов/CI/лицензии, дублям моделей, игнорированию ошибок. Фазы 0–1 закрывают почти все такие замечания |

## B.6. Первый шаг (рекомендация)

Начать с **Фазы 0 целиком** одним PR «chore: make the repository build and verify»:
удалить мусор, починить `cmd/*`, конфиг-example, Makefile, CI. Это ~5 часов и сразу даёт
зелёные бейджи, после чего дальше можно двигаться инкрементально и показывать прогресс.

---

## B.7. Отчёт о выполнении Фаз 0–1

### Фаза 0 — «здоровый клон»

- Удалены: корневой `main.go` (шаблон GoLand), `cmd/cron`, `internal/cron`, пустые файлы
  (`scripts/*`, `docker/Dockerfile.cron`, `deployments/systemd/cron.service`,
  `deployments/timeweb/setup.md`), мёртвые пакеты `pkg/*` (6 пакетов).
- `cmd/migrate` реализован: команды `up`, `down`, `status`.
- Конфиг: YAML + `${ENV}` + defaults + валидация всех обязательных полей сразу;
  `.env` больше не обязателен; `configs/config.yaml` закоммичен (без секретов) + `.env.example`.
- Добавлены `Makefile`, `.golangci.yml`, `.dockerignore`, GitHub Actions CI
  (`gofmt`, `build`, `vet`, `test -race`; `golangci-lint` — пока non-blocking).
- Docker: единый multi-stage `docker/Dockerfile` (bot + migrate, non-root, HEALTHCHECK),
  прод-compose с healthcheck'ами и сервисом `migrate` перед ботом, dev-compose только
  с инфраструктурой. Исправлен systemd-юнит (`/opt/mega-trainer`, `.env`, hardening).
- `go vet`: было 6 замечаний → 0. `gofmt`: 1 файл → 0.

### Фаза 1 — архитектура и корректность

- **Единый домен:** `domain.User`, `domain.UserState`, value-объекты расчётов,
  константы-словарь, типизированные ошибки `ErrNotFound/ErrDuplicate/ErrValidation`.
  Удалены дубли: `postgres.User`, `postgres.UserRepository`-интерфейс, `pkg/telegram.User`.
- **Один калькулятор** в `internal/service` (Mifflin–St Jeor); удалён дубль
  `pkg/calculator` (Harris–Benedict). Добавлена валидация входов и защита от `NaN/Inf`.
- **Один механизм миграций:** `migrations/NNNN_*.sql` + `.down.sql` + `embed.FS` +
  `Migrator` с таблицей `schema_migrations` и `pg_advisory_lock`; удалены хардкод
  `GetAllMigrations` и конфликтующий `migrations/001_create_users.sql`.
- **Composition root** в `cmd/bot`: конфиг → PostgreSQL → Redis → репозитории → сервисы →
  handler. БД реально используется (upsert пользователя на каждом update), `defer pg.Close()`
  из конструктора убран.
- **FSM без потери данных:** `StateService.Start/SetStep/SetData/Finish`; `SetStep`
  сохраняет собранные ответы (исправлен баг с затиранием выбранной формулы 1ПМ).
  Новая команда сбрасывает диалог, добавлен `/cancel`.
- **Конкурентность:** ограниченный пул воркеров (`app.workers`), per-user mutex
  (`keyedLocker`), `messageHistory` под мьютексом и с лимитом 20 сообщений на чат.
- **Устойчивость:** все ошибки `Send` проверяются, ошибки возвращаются из воркера и
  логируются; graceful shutdown по `SIGINT`/`SIGTERM` с ожиданием воркеров;
  `log.Fatal`/`panic` остались только в `main`.
- **Наблюдаемость:** `/healthz`, `/readyz` (проверяют PostgreSQL и Redis), `/metrics`
  с реально инкрементируемыми метриками; два набора метрик заменены одним.
- `pkg/*` удалён полностью: ни один пакет больше не импортирует `internal`.

### Проверки

```text
gofmt -l .            пусто
go build ./...        OK
go vet ./...          OK
go test -race ./...   OK
```

Покрытие: всего 47.2 %, `internal/service` — 87.8 %, `internal/config` — 83.3 %,
`internal/delivery/telegram` — 69.1 %, `internal/plot` — 84.6 %.

Тесты (без БД и Telegram): формулы и их валидация, `StateService`, `UserService`,
сквозные FSM-сценарии (`/water`, `/onerepmax`, `/tdee`, `/bodyfat`), устаревший callback,
сброс диалога командой, игнор групповых чатов, ошибка отправки, загрузка конфига,
рендер PNG-графика.

Smoke-проверки бинарников:

```text
$ ./bin/bot            # без env
fatal: invalid configuration: telegram.bot_token is required ...
       redis.address is required ...
       database.postgres.host is required ...

$ ./bin/migrate status # БД недоступна
fatal: ping postgres: dial tcp 127.0.0.1:5432: connect: connection refused
```

### Что осталось и ограничения проверки

- **Не проверено в этой среде:** `docker build` / `docker compose` (Docker не установлен),
  реальный прогон бота с Telegram-токеном и миграции на живой PostgreSQL.
  Перед следующим шагом это нужно прогнать у себя: `make docker-up`, `make run`.
- **Фаза 2:** `body_measurements`, `/chart` (рендер готов, но не подключён), `/history`,
  `/export`, `cmd/cron` + напоминания, бэкапы.
- **Фаза 3:** интеграционные тесты репозиториев и миграций (testcontainers-go),
  блокирующий `golangci-lint`, `LICENSE`, `CONTRIBUTING`, ADR, релиз/тег.
- Изменения лежат в рабочем дереве одним набором; перед пушем их стоит разбить на
  атомарные коммиты по Conventional Commits (см. B.4, п. 11).

## B.8. Обновление: прокси для Telegram и Фаза 2

### Поддержка прокси (требование РФ)

`api.telegram.org` недоступен напрямую, поэтому бот не мог авторизоваться:
`authorize telegram bot: ... dial tcp 149.154.166.110:443: connect: connection refused`.

- Новый пакет `internal/telegramapi`: собирает `http.Client` с HTTP(S)-прокси и создаёт
  клиент Bot API; при старте удаляет webhook (иначе `getUpdates` возвращает 409 и бот
  «молчит»).
- Конфиг: `telegram.proxy_url` ← `TELEGRAM_PROXY_URL` (например,
  `http://127.0.0.1:12334`). Поддерживаются только `http://` и `https://`: Bot API — это
  HTTPS, HTTP CONNECT-прокси достаточно, лишняя зависимость не нужна.
- Валидация URL на старте, маскирование креденшелов в логе, подсказка про прокси в тексте
  ошибки авторизации.
- `internal/telegramapi` покрыт unit-тестами (прокси-транспорт, отклонение socks5,
  маскирование).
- `.env` и `.env.example` обновлены; Dockerfile собирается из `golang:${GO_VERSION}`
  (`GO_VERSION` = версия из `toolchain` в `go.mod`).

### Фаза 2: дневник измерений и графики

- Миграция `0002_measurements.sql`: таблица `measurements` (вес, % жира, обхваты,
  `measured_at`) с индексом `(user_id, measured_at DESC)` и `ON DELETE CASCADE`.
- `domain.Measurement` + `MeasurementRepository`; реализация
  `internal/storage/postgres/measurements.go`.
- `service.MeasurementService`: резолвит Telegram-ID → внутренний `user_id`, валидирует
  вес, отдаёт историю и хронологический ряд веса для графика.
- Новые команды: `/measure`, `/chart` (PNG через `internal/plot` + `SendPhoto`),
  `/history`, `/export` (CSV через `SendDocument`).
- `/bodyfat` теперь автоматически сохраняет измерение (вес, % жира, обхваты) и предлагает
  `/chart`; ошибка записи логируется и не ломает показ результата.
- Обновлены меню `/start` и главного меню.

### Проверки после обновления

```text
gofmt -l .            пусто
go build ./...        OK
go vet ./...          OK
go test -race ./...   OK
```

Новые тесты: формула/валидация измерений, запись и чтение истории, порядок ряда для
графика, `/measure`, `/chart` (проверяется PNG-сигнатура вложения), `/history`, `/export`
(структура CSV), автосохранение после `/bodyfat`, валидация proxy URL.

### Осталось из Фазы 2 / Фазы 3

- Напоминания (`cmd/cron`) и бэкапы — не реализованы.
- Интеграционные тесты репозиториев и миграций на живой PostgreSQL.
- `golangci-lint` сделать блокирующим, `LICENSE`, `CONTRIBUTING`, ADR, релиз.
- Проверка на реальном Telegram через прокси — за владельцем (в среде разработки нет
  доступа к Bot API и к локальному прокси).

## B.9. Обновление: хвост Фазы 2 и Фаза 3

### Хвост Фазы 2

- **Напоминания:** `cmd/cron` + `internal/cron` — раз в `reminders.interval_hours`
  (default 24) рассылка активным пользователям (`UserRepository.ListActive`).
  Ошибка доставки одному пользователю не прерывает рассылку; ошибка БД логируется,
  следующий тик повторяет. Есть unit-тесты (рассылка, изоляция ошибок, остановка по
  контексту).
- **Бэкапы:** `scripts/backup.sh` — `pg_dump` через `docker compose exec`, gzip,
  ротация по `RETENTION_DAYS`, удаление неполного дампа при ошибке; `make backup`.
- Docker: образ собирает три бинарника (`bot`, `cron`, `migrate`), compose получил
  сервис `cron`; добавлен `deployments/systemd/cron.service`.

### Фаза 3

- `LICENSE` (MIT).
- `docs/ARCHITECTURE.md` — слои, mermaid-диаграммы потока update и схемы БД,
  таблица зависимостей пакетов, сознательные ограничения.
- `docs/OPERATIONS.md` — режимы запуска, миграции, напоминания, бэкап/восстановление,
  health/metrics, обновление, troubleshooting.
- `docs/TEST_PLAN.md` — уровни, матрица кейсов (расчёты, FSM, измерения,
  инфраструктура, безопасность), критерии приёмки.
- ADR: один калькулятор, FSM в Redis, embedded-миграции, HTTP(S)-прокси.
- CI/линтер: `golangci-lint-action@v7` + конфиг v2 (совместим с Go 1.27 из `go.mod`);
  Docker собирается из `golang:${GO_VERSION}-alpine`, `GO_VERSION=1.27.0` — синхронно
  с `toolchain go1.27`.
- README переписан в витрину: возможности, архитектура, быстрый старт, конфиг,
  прокси, напоминания/бэкапы, документация, roadmap, лицензия.

### Проверки

```text
gofmt -l .            пусто
go build ./...        OK   (toolchain go1.27)
go vet ./...          OK
go test -race ./...   OK
```

### Что осталось

- Интеграционные тесты репозиториев и миграций на живом PostgreSQL
  (testcontainers-go) — единственный заметный пробел в тестовой пирамиде.
- Релизный тег и прогон CI на GitHub.
- Ручной E2E по `docs/TEST_PLAN.md` (раздел 5) на реальном боте.

## B.10. Обновление: интеграционные и e2e-тесты, golangci-lint

### Что добавлено

- **Composition root вынесен в `internal/app`**: `cmd/bot` теперь только грузит конфиг,
  логгер и обрабатывает сигналы; объектный граф собирается в одном месте и переиспользуется
  e2e-тестами. Появились конфиг-опции `telegram.api_url` (подмена Bot API для тестов) и
  `database.postgres.search_path` (изоляция схем).
- **Интеграционные тесты** (`//go:build integration`, `internal/storage/postgres`):
  миграции up/down/идемпотентность, `UserRepo.Upsert`, `MeasurementRepo` (сортировка, лимит,
  NULL-поля, изоляция пользователей), `ON DELETE CASCADE`. Каждый тест работает в своей
  временной схеме PostgreSQL, поэтому данные разработчика не затрагиваются.
- **E2E-тесты** (`//go:build e2e`, `internal/app`): поднимается фейковый Telegram Bot API
  (`httptest`), реальные PostgreSQL и Redis, полный цикл `app.Run` (getMe → deleteWebhook →
  long polling). Сценарии: `/start`, диалог `/water` до результата, `/measure` ×2 →
  `/chart` (проверяется PNG), `/history`, `/export` (проверяется CSV), устаревший callback,
  неверный ввод. Проверяется graceful shutdown по отмене контекста.
- **golangci-lint**: установлен v2.15.0 (Go 1.27), конфиг переведён на формат v2,
  исправлены все 10 замечаний (gocritic appendAssign, shadowing `min/max`, unused-параметр,
  `fmt.Fprintf` вместо `WriteString(Sprintf)`, unparam в тестовых хелперах, package-comment).
  Джоба в CI теперь **блокирующая**.
- **CI** дополнен джобами `integration` и `e2e` с сервисами PostgreSQL 16 и Redis 7;
  Makefile получил `test-integration`, `test-e2e`, `test-all`, `lint` (gofmt + vet + линтер).
- **Документация**: README (уровни тестов и требования), `docs/TEST_PLAN.md`
  (матрица + карта автоматизации), AGENTS (DoD и правила тестов), ARCHITECTURE (internal/app).

### Проверки (выполнены локально на Go 1.27.0)

```text
gofmt / go build / go vet                  OK
go test -race ./...                        OK, покрытие unit 50.0 %
golangci-lint run                          OK, 0 issues
go test -tags=integration (PostgreSQL)     OK, 5 тестов (миграции, репозитории)
go test -tags=e2e (fake Bot API + PG + RD) OK, 6 сценариев, graceful shutdown
```

Интеграционные и e2e-тесты прогонялись против локального PostgreSQL 16 и Redis 7
(`make docker-up-dev`); в средах без инфраструктуры они корректно пропускаются
(`t.Skip`), если не заданы `TEST_POSTGRES_DSN` / `TEST_POSTGRES_*` / `TEST_REDIS_ADDR`.

### Что осталось

- Прогон CI на GitHub после пуша ветки `renew`.
- Ручной E2E в реальном Telegram (раздел 5 `docs/TEST_PLAN.md`) — за владельцем.
- Релизный тег (`v1.0.0`) после приёмки MR.

---

_Документ поддерживается в актуальном состоянии: при закрытии задач фазы отмечайте
прогресс в README/Roadmap._
