-- Восстановить активный больничный для legacy-пользователей (pack-row training_state).
--
-- Стейт как после activateSickLeave в боте:
--   has_sick_leave = true, has_healthy = false, таймер кика не восстанавливается до #healthy.
--
-- ✏️  pack_chat_id = MONETIZED_CHAT_ID
-- ✏️  блок INSERT INTO _legacy_sick_leave
--
-- ⚠️  После применения перезапусти ms_leo — иначе in-memory таймер кика мог остаться активным.
--
-- Запуск:
--   psql "$DATABASE_URL" -f scripts/set_legacy_sick_leave.sql

ROLLBACK;
BEGIN;

CREATE TEMP TABLE _import_cfg AS
SELECT -1001234567890::bigint AS pack_chat_id;  -- MONETIZED_CHAT_ID

CREATE TEMP TABLE _legacy_sick_leave (
    user_id                         BIGINT PRIMARY KEY,
    username                        TEXT NOT NULL DEFAULT '',
    sick_leave_start_time           TEXT NOT NULL,  -- RFC3339, Europe/Moscow
    timezone_offset_from_moscow     INT  NOT NULL DEFAULT 0,
    message_text                    TEXT NOT NULL
);

INSERT INTO _legacy_sick_leave (
    user_id, username, sick_leave_start_time, timezone_offset_from_moscow, message_text
) VALUES (
    100970059,
    '@johny_taker',
    '2026-05-27T19:41:00+03:00',  -- 17:41 местное (MSK−2)
    -2,
    '#sick_leave Падение на велосипеде. Сильный удар, еле передвигаюсь'
);

UPDATE training_state ts
SET
    has_sick_leave              = TRUE,
    has_healthy                 = FALSE,
    sick_leave_start_time       = sl.sick_leave_start_time,
    sick_leave_end_time         = NULL,
    sick_time                   = NULL,
    sick_approval_pending       = FALSE,
    sick_approval_deadline      = NULL,
    sick_approval_message_id    = NULL,
    timezone_offset_from_moscow = sl.timezone_offset_from_moscow,
    updated_at                  = timezone('Europe/Moscow', now())
FROM _legacy_sick_leave sl
CROSS JOIN _import_cfg cfg
WHERE ts.user_id = sl.user_id
  AND ts.chat_id = cfg.pack_chat_id;

INSERT INTO user_messages (user_id, chat_id, username, message_text, message_type, created_at)
SELECT
    sl.user_id,
    sl.user_id,
    COALESCE(NULLIF(BTRIM(sl.username), ''), NULLIF(BTRIM(st.username), ''), ''),
    sl.message_text,
    'sick_leave',
    sl.sick_leave_start_time::timestamptz
FROM _legacy_sick_leave sl
CROSS JOIN _import_cfg cfg
LEFT JOIN training_state st
    ON st.user_id = sl.user_id AND st.chat_id = cfg.pack_chat_id
WHERE NOT EXISTS (
    SELECT 1 FROM user_messages um
    WHERE um.user_id = sl.user_id
      AND um.chat_id = sl.user_id
      AND um.message_type = 'sick_leave'
      AND um.message_text = sl.message_text
);

SELECT
    sl.user_id,
    sl.username,
    st.chat_id AS pack_chat_id,
    st.has_sick_leave,
    st.has_healthy,
    st.sick_leave_start_time,
    st.timezone_offset_from_moscow,
    st.timer_start_time,
    (
        SELECT COUNT(*)::INT FROM user_messages um
        WHERE um.user_id = sl.user_id AND um.message_type = 'sick_leave'
    ) AS sick_leave_messages
FROM _legacy_sick_leave sl
CROSS JOIN _import_cfg cfg
LEFT JOIN training_state st
    ON st.user_id = sl.user_id AND st.chat_id = cfg.pack_chat_id;

COMMIT;
