-- Полная очистка ленты стаи и общего чата (только UGC, без профилей/стриков/кубков).
--
-- ✏️  pack_chat_id = MONETIZED_CHAT_ID
--
-- ⚠️  Без подтверждения — для ручного запуска в psql/pgAdmin.
--     В боте безопаснее: /admin → «🗑 Очистить ленту и чат» → подтверждение.
--
-- Запуск:
--   psql "$DATABASE_URL" -f scripts/clear_pack_feed_and_chat.sql

ROLLBACK;
BEGIN;

CREATE TEMP TABLE _wipe_cfg AS
SELECT *
FROM (VALUES
    (-1001234567890::bigint)  -- ← MONETIZED_CHAT_ID
) AS t(pack_chat_id);

SELECT
    'before' AS phase,
    (SELECT COUNT(*) FROM user_messages um
     WHERE um.chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_posts,
    (SELECT COUNT(*) FROM miniapp_training_feed_thread t
     WHERE t.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_threads,
    (SELECT COUNT(*) FROM miniapp_feed_reports r
     WHERE r.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_reports,
    (SELECT COUNT(*) FROM miniapp_pack_group_chat g
     WHERE g.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS pack_chat_messages;

DELETE FROM miniapp_training_thread_unread
WHERE thread_reply_id IN (
    SELECT t.id FROM miniapp_training_feed_thread t
    WHERE t.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)
);

DELETE FROM miniapp_training_feed_thread_likes
WHERE thread_reply_id IN (
    SELECT t.id FROM miniapp_training_feed_thread t
    WHERE t.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)
);

DELETE FROM miniapp_training_feed_thread
WHERE pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

DELETE FROM miniapp_training_feed_reactions
WHERE pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

DELETE FROM miniapp_feed_poll_votes
WHERE user_message_id IN (
    SELECT um.id FROM user_messages um
    WHERE um.chat_id = (SELECT pack_chat_id FROM _wipe_cfg)
);

DELETE FROM miniapp_feed_reports
WHERE pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

DELETE FROM user_messages
WHERE chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

DELETE FROM miniapp_pack_group_unread
WHERE pack_message_id IN (
    SELECT g.id FROM miniapp_pack_group_chat g
    WHERE g.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)
);

DELETE FROM miniapp_pack_group_reactions
WHERE pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

DELETE FROM miniapp_pack_group_chat
WHERE pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg);

SELECT
    'after' AS phase,
    (SELECT COUNT(*) FROM user_messages um
     WHERE um.chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_posts,
    (SELECT COUNT(*) FROM miniapp_training_feed_thread t
     WHERE t.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_threads,
    (SELECT COUNT(*) FROM miniapp_feed_reports r
     WHERE r.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS feed_reports,
    (SELECT COUNT(*) FROM miniapp_pack_group_chat g
     WHERE g.pack_chat_id = (SELECT pack_chat_id FROM _wipe_cfg)) AS pack_chat_messages;

COMMIT;
