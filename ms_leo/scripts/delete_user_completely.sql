-- Полное удаление пользователя из БД ms_leo (PostgreSQL).
-- Работает в pgAdmin, DBeaver, Railway SQL, psql.
--
-- Поставь Telegram user_id в объявление target_user_id ниже и выполни весь файл.
--
-- НЕ трогает: Qdrant/RAG, Telegram, БД leo_payments — см. комментарии в конце.

BEGIN;

DO $delete_user$
DECLARE
  -- ═══ Telegram user_id (единственное место для правки) ═══
  target_user_id bigint := 123456789;
BEGIN
  IF target_user_id IS NULL OR target_user_id <= 0 THEN
    RAISE EXCEPTION 'target_user_id должен быть положительным bigint, сейчас: %', target_user_id;
  END IF;

  -- Превью (раскомментируй внутри блока перед DELETE):
  -- RAISE NOTICE 'training_state: %', (SELECT COUNT(*) FROM training_state WHERE user_id = target_user_id);
  -- RAISE NOTICE 'user_messages: %', (SELECT COUNT(*) FROM user_messages WHERE user_id = target_user_id);

  CREATE TEMP TABLE _del_user_msg_ids ON COMMIT DROP AS
  SELECT id FROM user_messages WHERE user_id = target_user_id;

  CREATE TEMP TABLE _del_thread_ids ON COMMIT DROP AS
  SELECT t.id
  FROM miniapp_training_feed_thread t
  WHERE t.from_user_id = target_user_id
     OR t.user_message_id IN (SELECT id FROM _del_user_msg_ids);

  DELETE FROM miniapp_training_thread_unread u
  WHERE u.recipient_user_id = target_user_id
     OR u.thread_reply_id IN (SELECT id FROM _del_thread_ids);

  DELETE FROM miniapp_training_feed_thread_likes l
  WHERE l.user_id = target_user_id
     OR l.thread_reply_id IN (SELECT id FROM _del_thread_ids);

  DELETE FROM miniapp_training_feed_thread t
  WHERE t.id IN (SELECT id FROM _del_thread_ids);

  DELETE FROM miniapp_training_feed_reactions r
  WHERE r.user_id = target_user_id
     OR r.user_message_id IN (SELECT id FROM _del_user_msg_ids);

  DELETE FROM miniapp_feed_poll_votes v
  WHERE v.user_id = target_user_id;

  DELETE FROM user_messages WHERE user_id = target_user_id;
  DELETE FROM training_sessions WHERE user_id = target_user_id;

  DELETE FROM miniapp_personal_chat_likes l
  USING miniapp_personal_chat c
  WHERE l.message_id = c.id AND c.user_id = target_user_id;

  DELETE FROM miniapp_personal_chat_likes
  WHERE user_id = target_user_id;

  DELETE FROM miniapp_personal_chat WHERE user_id = target_user_id;
  DELETE FROM miniapp_support_chat WHERE user_id = target_user_id;
  DELETE FROM miniapp_pack_group_chat WHERE from_user_id = target_user_id;

  DELETE FROM miniapp_user_profile WHERE user_id = target_user_id;
  DELETE FROM paywall_access_requests WHERE user_id = target_user_id;
  DELETE FROM deletion_events WHERE user_id = target_user_id;

  DELETE FROM outbox_events o
  WHERE NULLIF(o.payload->>'UserID', '')::bigint = target_user_id
     OR NULLIF(o.payload->>'user_id', '')::bigint = target_user_id;

  DELETE FROM training_state WHERE user_id = target_user_id;

  IF to_regclass('public.training_log') IS NOT NULL THEN
    DELETE FROM training_log WHERE user_id = target_user_id;
  END IF;

  RAISE NOTICE 'OK: удалён user_id = %', target_user_id;
END;
$delete_user$;

COMMIT;

-- Qdrant: session_id = 'personal:<target_user_id>:<pack_chat_id>'
-- leo_payments: DELETE FROM yookassa_payment_events WHERE user_telegram_id = <target_user_id>;
