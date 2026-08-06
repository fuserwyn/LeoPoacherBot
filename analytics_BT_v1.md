---
artifact_type: technical_requirements
title: "БТ Аналитика приложения — события, воронки, метрики"
version: 1.0
status: review
owner: cpo
role_executed_by: cpo_session
compromise_note: "Бизнес-требования к аналитике от CPO. Расширяет минимум из DM-E-activation-analytics (3 события для kill-метрики) до полного analytics-БТ под альфа+F1. Содержит: event schema, 3 ключевые воронки, метрики strategy KPI, технические требования к сторе/тулу, post-launch retrospective. Передаётся backend как спецификация."
supersedes_partial: "[[DM-2026-05-13-E-activation-analytics]]"
parent_artifacts:
  - "[[DM-2026-05-13-E-activation-analytics]]"
  - "[[fat_leopard_strategy_v3_3]]"
  - "[[fat_leopard_brd_v1_3]]"
  - "[[wolf_pack_rules_v1_0]]"
  - "[[D-2026-05-20-ceo-decisions-wave1]]"
priority: P0
phase: F1
due: 2026-05-31
tags: [analytics, BT, requirements, funnel, kpi, backend_spec]
location: "20_work/cpo/"
---

# БТ — Аналитика приложения Fat Leopard

> **Цель:** определить, что трекать, чтобы измерить успех F1 и принять решение go/no-go для масштабирования. Документ — спецификация для backend, не техническая реализация.

## 1. Принципы

- **Минимум для F1**, но достаточный для всех ключевых решений (kill-метрика, фидбек на воронке оплаты, retention)
- **Server-side трекинг** (Edge Functions, webhook'и) — никакой client-side trust для финансовых событий
- **Идемпотентность** — одно событие = один раз, даже при ретраях
- **Простое хранилище в F1** — одна таблица `events` в БД (PostgreSQL/Supabase). НЕ Amplitude/Mixpanel/PostHog (избыточно для F1, дорого, latency)
- **F2 миграция** при росте — переедем в спец-тул когда объём оправдает

## 2. Event Schema (БД)

Таблица `events` в основной БД:

```sql
events (
  id           BIGSERIAL PRIMARY KEY,
  event_name   VARCHAR(64) NOT NULL,
  user_id      BIGINT,              -- nullable для пред-регистрационных событий
  telegram_id  BIGINT,
  occurred_at  TIMESTAMPTZ NOT NULL,
  payload      JSONB,                -- свободные параметры события
  session_id   UUID,                 -- для группировки в сессию
  source       VARCHAR(32),          -- utm_source / channel attribution
  app_version  VARCHAR(16),
  created_at   TIMESTAMPTZ DEFAULT NOW()
)
```

Индексы: `(event_name, occurred_at)`, `(user_id, occurred_at)`, `(telegram_id, event_name)`.

**Retention:** raw events — 365 дней. Дальше — агрегированные таблицы (DAU/WAU/MAU snapshots).

## 3. Воронка 1 — БОТ (приобретение и оплата)

**Это главная воронка ты просил: старт → оплата → миниапп.**

```
bot_started → paywall_viewed → payment_initiated → payment_completed → miniapp_opened
```

> **С бесплатным входом (`PAYWALL_ENTRY_FREE=true`) воронка укорачивается: `bot_started → miniapp_opened`.**
> События `paywall_*` / `payment_*` остаются, но описывают только платный возврат выбывших за
> неактивность. Добровольная поддержка живёт отдельно: `donate_initiated` → `donate_completed`
> (payload `provider: 'stars' | 'yukassa' | 'card'`, `amount_minor`, `currency`; эмиттер — ms_leo,
> idempotency key по id доната). Готовность платить считаем по донатам и по возвратам, а не по входу.

| Событие | Когда | Payload (ключевое) |
|---|---|---|
| `bot_started` | `/start` в боте | `source` (deep-link utm), `is_returning` (был ли раньше) |
| `paywall_viewed` | Юзер увидел экран Шаг 2 (выбор способа оплаты) | — |
| `payment_method_selected` | Нажал «Карта РФ» или «Звёзды» | `provider: 'yukassa' \| 'stars'` |
| `payment_initiated` | Перешёл на форму оплаты | `provider`, `amount` |
| `payment_completed` | Webhook успешной оплаты | `provider`, `amount`, `payment_id`, `cohort_position` |
| `payment_failed` | Webhook fail / отмена | `provider`, `reason` (если есть) |
| `welcome_message_sent` | Шаг 4 «Ура, ты в Стае» отправлено | — |
| `miniapp_opened` | Юзер открыл Mini App впервые | `entry_point: 'welcome_button' \| 'profile' \| 'other'` |

**Метрики воронки 1:**
- Конверсия `bot_started → paywall_viewed` (% дошёл до экрана оплаты)
- Конверсия `paywall_viewed → payment_initiated` (% выбрал способ)
- **Конверсия `paywall_viewed → payment_completed` ⭐ ключевая** — готовность платить
- Drop-off по `payment_method_selected` (карта vs звёзды)
- Конверсия `payment_completed → miniapp_opened` (доходят ли до миниаппа после оплаты)
- Time-to-payment: медиана `bot_started → payment_completed`

## 4. Воронка 2 — АКТИВАЦИЯ (первая тренировка)

```
miniapp_opened → workout_log_started → workout_logged → leo_comment_received
```

| Событие | Когда | Payload |
|---|---|---|
| `workout_log_started` | Юзер нажал «+» (открыл форму логирования) | — |
| `workout_logged` | Тренировка сохранена | `activity_type`, `duration_min`, `intensity`, `is_first_lifetime`, `cups_awarded` |
| `leo_comment_received` | Промт 1 вернул комментарий | `latency_ms`, `prompt_version` |
| `leo_comment_displayed` | Юзер увидел комментарий в карточке | — |

**Метрики воронки 2 (Activation):**
- **`first_workout_logged / payment_completed` ⭐ kill-метрика** (BRD ≥35%, kill <20%)
- Time-to-first-workout: median, p50, p95 от `payment_completed`
- Distribution `activity_type` первых тренировок (валидация: попадаем ли в нашу аудиторию или это люди с HIIT)

## 5. Воронка 3 — RETENTION И СЕРИИ

| Событие | Когда | Payload |
|---|---|---|
| `streak_incremented` | Стрик вырос (тренировка в новый день) | `new_streak_days`, `prev_streak_days` |
| `streak_attempt_used` | Попытка спасла стрик | `attempts_left` |
| `streak_broken` | Стрик обнулился | `streak_lost_days` |
| `level_up` | Достиг нового уровня | `level_from`, `level_to`, `cups_total` |
| `milestone_achieved` | Стрик достиг ачивки (7/14/30/42/60/90/180/365) | `milestone_days`, `is_record` |
| `burn_warning_sent` | Лео отправил day_5/6/7 alert | `stage: 'day_5' \| 'day_6' \| 'day_7'` |
| `burn_recovered` | Тренировка после burn alert (стрик удалось спасти) | `was_at_stage` |
| `sick_leave_started` | Активировал больничный | — |
| `sick_leave_ended` | Снял больничный (выздоровел/тренировка) | `via: 'manual' \| 'training' \| 'auto_14d'`, `days_used` |
| `account_deleted_inactivity` | Авто-удаление на день 8 | `last_streak`, `lifetime_workouts` |
| `account_reactivated` | Вернулся после удаления (новый платёж) | `days_since_deletion`, `lifetime_workouts` |

**Метрики воронки 3:**
- **NSM ⭐ ключевая (Strategy §09):** % юзеров со стриком ≥7 на 14-й день (target >50%, kill <20%)
- **D7 retention** (BRD target >25%, kill <15%)
- **D30 retention** (target >10%)
- Burn recovery rate: `burn_recovered / burn_warning_sent` (по стадиям)
- Реактивация: `account_reactivated / account_deleted_inactivity` (% возвращаются → сила привычки)
- Среднее использование больничного: дней/раз

## 6. Дополнительные события (UGC, Лео, модерация)

| Событие | Когда | Payload |
|---|---|---|
| `feed_reaction_added` | Реакция в ленте | `target_type: 'workout' \| 'comment'`, `reaction_type` |
| `feed_comment_posted` | Комментарий в ленте | `length` |
| `leo_chat_message_sent` | Юзер написал Лео в чат | `chat_messages_today` (rate-limit visibility) |
| `leo_chat_limit_reached` | Хит 20/день лимита | — |
| `moderation_blocked` | PRE-фильтр заблокировал контент | `surface: 'workout_note' \| 'feed_comment' \| 'leo_chat'`, `reason: 'profanity' \| 'url' \| 'illegal' \| 'length' \| 'rate_limit'` |
| `complaint_filed` | Жалоба через кнопку | `target_type`, `target_id` |
| `report_resolved` | Админ закрыл жалобу | `action: 'hide' \| 'warn' \| 'mute' \| 'ban' \| 'no_action'` |
| `support_button_clicked` | Кнопка «Поддержка» | `from: 'bot' \| 'miniapp'` |

## 7. Сводка ключевых метрик (привязка к strategy KPI)

| Метрика | Источник | Target (Strategy §09) | Kill |
|---|---|---|---|
| **NSM** — % стрик≥7 на 14д | streak_incremented + cohort | >50% | <20% |
| **Activation** — first_workout / paid | воронка 2 | >60% (BRD) / >35% (Strategy) | <35% / <20% |
| **D7 retention** | active vs first_visit | >25% | <15% |
| **D30 retention** | active vs first_visit | >10% | — |
| **Конверсия paywall→paid** | воронка 1 | требует валидации (нет baseline) | — |
| **k-factor (вирусность)** | source attribution + invites | >0.2 | — |
| **Organic %** | source dimension | >40% | — |
| **Sean Ellis score** | отдельный опрос (PMM анкета) | >40% disappointed | — |

## 8. Channel attribution (для CMO)

Каждый `bot_started` несёт `source` из deep-link параметра. UTM-формат:
```
https://t.me/fatleopardbot?start=src-tg_channel_main
https://t.me/fatleopardbot?start=src-personal_dm
https://t.me/fatleopardbot?start=src-organic
```

Парсится при `bot_started`, сохраняется в payload. **Без этого CMO не сможет атрибутировать каналы при F2 платном маркетинге.**

## 9. Технические требования

### 9.1 Эмиттеры (где пишутся события)
- Telegram-бот (Edge Function): `bot_started`, `paywall_viewed`, `payment_method_selected`, `welcome_message_sent`
- Payment webhook (ЮКасса + Telegram Stars): `payment_initiated`, `payment_completed`, `payment_failed`
- Mini App backend: `miniapp_opened`, `workout_log_started`, `workout_logged`, `streak_*`, `level_up`, `milestone_achieved`, все UGC и Лео-события
- Cron-job: `burn_warning_sent`, `account_deleted_inactivity` (по локальному TZ юзера)
- Модерация-гейт: `moderation_blocked`

### 9.2 Надёжность
- Запись событий — **non-blocking** (асинхронно, не должна мешать основному flow)
- При недоступности БД — буфер в очереди (Redis/in-memory), retry с экспоненциальным backoff
- Дублирование: каждое финансовое событие имеет idempotency key (payment_id), повторный webhook не плодит дубли

### 9.3 PII и согласие
- `telegram_id` — это персональные данные (152-ФЗ)
- Согласие даётся при акцепте оферты (раздел 6 оферты)
- Retention: 365 дней raw → анонимизация при удалении аккаунта (telegram_id → hash)

### 9.4 Анализ в F1
- SQL-запросы вручную из БД (psql/Supabase Studio)
- **Опционально:** простой dashboard в Metabase/Superset (open-source, self-hosted, 1-2 дня) для базовых графиков
- Полноценный BI — F2

## 10. Альфа-специфика (тестеры из своих)

Для альфа-когорты:
- Флаг `is_alpha_tester: true` на user (отметить вручную или через source)
- Отдельная агрегация метрик по альфе vs прод
- **Главное в альфе:** конверсия в первое логирование + тон Лео (через ручной review `leo_comment_received` payload)

## 11. Post-launch retrospective (через 28 дней)

Минимальный отчёт:
1. Воронка приобретения (полностью)
2. Activation (первая тренировка)
3. NSM cohort (как менялся к 14 дню)
4. Distribution `activity_type` (валидация ABCDX-сегмента A)
5. Channel ROI (если CMO привлекал)
6. Burn recovery rate
7. Реактивация

Это — основа go/no-go для расширения и F2 планирования.

## 12. Open questions для backend

1. Где хранится таблица `events` — в основной БД или отдельной? (CPO рекомендация: основная БД F1, отдельная — F2)
2. Готовность к Metabase/Superset — кто настраивает, есть ли ресурс? (опц. для F1)
3. UTM-формат deep-link — окончательный синтаксис согласовать с CMO

## Статус
```yaml
status: review
готовность: спецификация → передать backend на 28-31 мая
расширяет: DM-E (3 minimum events) до production-grade аналитики (~20 событий, 3 воронки)
блокеры_F1: воронка 1 (приобретение), воронка 2 (активация) — для kill-метрики и фидбека CMO
nice_to_have_F1: воронка 3 retention частично (NSM критично, остальное может догнать)
F2: миграция в Amplitude/PostHog, BI-tool, k-factor metric
```
