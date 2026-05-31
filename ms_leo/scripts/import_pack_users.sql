-- Ручной перенос пользователей из старого приложения.
--
-- ✏️  Редактируй ТОЛЬКО блок «ЕДИНСТВЕННОЕ МЕСТО ДЛЯ ПРАВОК» ниже.
--     Список user_id автоматически попадёт во все три таблицы.
--
-- Запуск:  psql "$DATABASE_URL" -f scripts/import_pack_users.sql

BEGIN;

-- =============================================================================
-- ✏️  ЕДИНСТВЕННОЕ МЕСТО ДЛЯ ПРАВОК
-- =============================================================================
-- 1) Одна строка настроек: pack_chat_id | skip_paywall | start_timer
--    skip_paywall = TRUE  → не трогать paywall (если PAYWALL_ENABLED=false)
--    start_timer  = FALSE → таймер неактивности при первом заходе в miniapp
--
-- 2) Список Telegram user_id — по одному на строку.
--    Имена не пишем: при первом заходе в miniapp бот подставит @username или имя из Telegram.

CREATE TEMP TABLE _import_cfg AS
SELECT *
FROM (VALUES
    (-1001234567890::bigint, FALSE, TRUE)
) AS t(pack_chat_id, skip_paywall, start_timer);

CREATE TEMP TABLE _import_users AS
SELECT user_id, NULL::text AS username
FROM (VALUES
    (600000::bigint),

) AS t(user_id);

-- =============================================================================
-- Дальше ничего не меняй — INSERT'ы берут данные из таблиц выше
-- =============================================================================

INSERT INTO paywall_access_requests (
    user_id, monetized_chat_id, status, completed_at, access_expires_at
)
SELECT
    u.user_id,
    c.pack_chat_id,
    'completed',
    NOW(),
    'infinity'::timestamptz
FROM _import_users u
CROSS JOIN _import_cfg c
WHERE NOT c.skip_paywall
  AND NOT EXISTS (
      SELECT 1
      FROM paywall_access_requests par
      WHERE par.user_id = u.user_id
        AND par.monetized_chat_id = c.pack_chat_id
        AND par.status = 'completed'
        AND par.access_expires_at IS NOT NULL
        AND par.access_expires_at > NOW()
  );

WITH now_msk AS (
    SELECT to_char(timezone('Europe/Moscow', now()), 'YYYY-MM-DD"T"HH24:MI:SS') || '+03:00' AS ts
)
INSERT INTO training_state (
    user_id,
    username,
    chat_id,
    streak_days,
    max_streak_days,
    cups_earned,
    last_message,
    has_training_done,
    has_sick_leave,
    has_healthy,
    is_deleted,
    timer_start_time,
    timezone_offset_from_moscow,
    achievement_count,
    return_count,
    created_at,
    updated_at
)
SELECT
    u.user_id,
    NULLIF(BTRIM(u.username), ''),
    c.pack_chat_id,
    0, 0, 0,
    '',
    FALSE, FALSE, FALSE, FALSE,
    CASE WHEN c.start_timer THEN n.ts ELSE NULL END,
    0, 0, 1,
    timezone('Europe/Moscow', now()),
    timezone('Europe/Moscow', now())
FROM _import_users u
CROSS JOIN _import_cfg c
CROSS JOIN now_msk n
ON CONFLICT (user_id, chat_id) DO NOTHING;

INSERT INTO miniapp_user_profile (
    user_id, pack_chat_id, gender, display_name, age_years, updated_at
)
SELECT
    u.user_id,
    c.pack_chat_id,
    '',
    COALESCE(NULLIF(BTRIM(u.username), ''), ''),
    NULL,
    NOW()
FROM _import_users u
CROSS JOIN _import_cfg c
ON CONFLICT (user_id, pack_chat_id) DO NOTHING;

-- Отчёт
SELECT 'paywall (активные completed)' AS what, COUNT(*) AS cnt
FROM paywall_access_requests par
JOIN _import_users u ON u.user_id = par.user_id
JOIN _import_cfg c ON c.pack_chat_id = par.monetized_chat_id
WHERE par.status = 'completed'
  AND par.access_expires_at IS NOT NULL
  AND par.access_expires_at > NOW()

UNION ALL

SELECT 'training_state', COUNT(*)
FROM training_state ts
JOIN _import_users u ON u.user_id = ts.user_id
JOIN _import_cfg c ON c.pack_chat_id = ts.chat_id
WHERE ts.is_deleted = FALSE

UNION ALL

SELECT 'miniapp_user_profile', COUNT(*)
FROM miniapp_user_profile p
JOIN _import_users u ON u.user_id = p.user_id
JOIN _import_cfg c ON c.pack_chat_id = p.pack_chat_id;

COMMIT;
