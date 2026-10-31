# Архитектура

Документ описывает, как устроен `mega-trainer-go`, какие у слоёв правила и как
проходит обработка одного сообщения.

## Слои и зависимости

Зависимости направлены только внутрь. `domain` не знает ни об одной инфраструктуре.

```mermaid
graph LR
    subgraph entrypoints
        BOT[cmd/bot]
        MIG[cmd/migrate]
        CRON[cmd/cron]
    end

    subgraph delivery
        TG[internal/delivery/telegram]
    end

    subgraph business
        SVC[internal/service]
        DOM[internal/domain]
    end

    subgraph infrastructure
        PG[internal/storage/postgres]
        RDS[internal/storage/redis]
        TAPI[internal/telegramapi]
        MON[internal/monitoring]
        LOG[internal/logger]
        CFG[internal/config]
    end

    BOT --> APP[internal/app]
    APP --> TG
    APP --> SVC
    BOT --> PG
    BOT --> RDS
    BOT --> TAPI
    BOT --> MON
    BOT --> LOG
    BOT --> CFG
    TG --> SVC
    SVC --> DOM
    PG --> DOM
    RDS --> DOM
    CRON --> DOM
    CRON --> TAPI
    MIG --> PG
```

| Пакет | Ответственность | Может импортировать |
|---|---|---|
| `internal/domain` | сущности, value-объекты, интерфейсы репозиториев, sentinel-ошибки | только stdlib |
| `internal/service` | формулы, FSM-хелперы, пользователи, измерения | `domain` |
| `internal/storage/*` | реализации интерфейсов `domain` поверх PostgreSQL/Redis | `domain`, `config` |
| `internal/delivery/telegram` | разбор update, диалоги, клавиатуры, отправка | `domain`, `service`, `plot` |
| `internal/telegramapi` | клиент Bot API, HTTP(S)-прокси, удаление webhook | `config` |
| `internal/cron` | фоновые задачи (напоминания) | `domain` |
| `internal/app` | composition root: собрать зависимости и запустить бота | всё |
| `cmd/*` | тонкие entrypoints: конфиг, логгер, сигналы | `config`, `logger`, `app` |

`pkg/` намеренно отсутствует: наружу ничего не публикуется, а импорт `internal`
из `pkg` был бы нарушением направления зависимостей.

## Обработка сообщения

```mermaid
sequenceDiagram
    participant TG as Telegram
    participant API as telegramapi
    participant BOT as internal/bot
    participant H as delivery/telegram
    participant S as service
    participant R as Redis
    participant PG as PostgreSQL

    TG->>API: getUpdates (long polling, через прокси)
    API-->>BOT: update
    BOT->>BOT: ограниченный пул воркеров
    BOT->>H: HandleUpdate(ctx, update)
    H->>H: keyed lock по пользователю
    H->>PG: Upsert user (telegram_id)
    H->>R: GetState(user)
    alt есть активный диалог
        H->>S: расчёт / следующий шаг
        H->>R: SetState / ClearState
    else команда
        H->>S: расчёт
    end
    H->>TG: Send (текст / фото / документ)
```

Ключевые решения:

- **Пул воркеров** вместо горутины на каждый update — защита от штормов и
  исчерпания ресурсов.
- **keyed lock по Telegram-ID** — сообщения одного пользователя обрабатываются
  последовательно, поэтому шаги FSM не перетирают друг друга.
- **FSM в Redis** с TTL 24 часа — бот переживает рестарт, состояние не копится
  в памяти.
- **Ошибки `Send` проверяются**; ошибка возвращается в воркер, логируется и
  инкрементирует `telegram_updates_failed_total`.

## Данные

```mermaid
erDiagram
    users ||--o{ measurements : "user_id"
    users {
        bigserial id PK
        bigint telegram_id UK
        text username
        text first_name
        text last_name
        text language_code
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }
    measurements {
        bigserial id PK
        bigint user_id FK
        double weight_kg
        double body_fat_pct
        double neck_cm
        double waist_cm
        double hips_cm
        timestamptz measured_at
        timestamptz created_at
    }
```

Схема живёт только в [`migrations/`](migrations/0001_init.sql), применяется
раннером из `cmd/migrate` (см. [ADR-0003](adr/0003-embedded-sql-migrations.md)).

## Конфигурация

`configs/config.yaml` + подстановка `${ENV}`. Валидация на старте собирает все
проблемы сразу (`errors.Join`), а не падает на первой. Секреты — только из
окружения; в конфиге — ссылки на переменные.

## Наблюдаемость

- `GET /healthz` — процесс жив.
- `GET /readyz` — проверяет PostgreSQL и Redis.
- `GET /metrics` — Prometheus: обработанные/упавшие update и гистограмма времени.
- Логи — zerolog, структурированные. Формат сообщения: `[компонент:функция] описание
  на русском`, контекст — в скобках; префикс собирается через `logger.Source`.
  Токены и прокси-креденшелы не логируются.

## Фоновые задачи

`cmd/cron` — отдельный процесс: раз в `reminders.interval_hours` рассылает
активным пользователям напоминание записать вес. Ошибка доставки одному
пользователю не прерывает рассылку; ошибка чтения БД логируется, и следующий
тик повторяет попытку.

## Что осознанно не сделано (и почему)

- **Webhook-режим** — не нужен для одного инстанса; long polling проще и не
  требует публичного HTTPS. При старте webhook удаляется, чтобы не было 409.
- **ORM** — `sqlx` + явный SQL: предсказуемо, без магии и скрытых запросов.
- **Кэш графиков** — рендер PNG занимает миллисекунды, кэширование усложнило бы
  инвалидацию без пользы на текущих объёмах.
