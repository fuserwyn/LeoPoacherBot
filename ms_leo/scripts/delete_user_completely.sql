-- Полное удаление пользователя из БД ms_leo (PostgreSQL).
-- Работает в pgAdmin, DBeaver, Railway SQL, psql.
--
-- Поставь Telegram user_id в объявление target_user_id ниже и выполни весь файл.
--
-- После скрипта /start должен снова требовать оплату (экран paywall), а не «Ура, ты в стае».
-- Важно:
--   1) Выполняй на той же БД, что DATABASE_URL у ms_leo (и ms_payments, если он пишет в неё).
--   2) Сначала чистим outbox (иначе воркер ms_leo между DELETE paywall и DELETE outbox
--      может снова выдать доступ по paywall_access_restore_requested).
--   3) Отдельно очисти leo_payments (см. блок в конце) — иначе повторный вебхук ЮKassa
--      может закрыть новую pending-заявку после /start.
--
-- НЕ трогает: Qdrant/RAG, Telegram.

BEGIN;

DO $delete_user$
DECLARE
  -- ═══ Telegram user_id (единственное место для правки) ═══
  target_user_id bigint := 7738691355;
  n_paywall_active bigint;
  n_training bigint;
  n_outbox bigint;
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

  CREATE TEMP TABLE _del_paywall_req_ids ON COMMIT DROP AS
  SELECT p.id
  FROM paywall_access_requests p
  WHERE p.user_id = target_user_id;

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
  DELETE FROM deletion_events WHERE user_id = target_user_id;

  -- Outbox до paywall: иначе воркер успевает «восстановить» доступ по старому событию.
  DELETE FROM outbox_events o
  WHERE o.event_type IN ('paywall_access_restore_requested', 'refund_requested')
    AND (
          (
            NULLIF(o.payload->>'UserID', '') ~ '^[0-9]+$'
            AND (o.payload->>'UserID')::bigint = target_user_id
          )
       OR (
            NULLIF(o.payload->>'user_id', '') ~ '^[0-9]+$'
            AND (o.payload->>'user_id')::bigint = target_user_id
          )
       OR (
            NULLIF(o.payload->>'request_id', '') ~ '^[0-9]+$'
            AND (o.payload->>'request_id')::bigint IN (SELECT id FROM _del_paywall_req_ids)
          )
        );

  DELETE FROM outbox_events o
  WHERE (
          NULLIF(o.payload->>'UserID', '') ~ '^[0-9]+$'
          AND (o.payload->>'UserID')::bigint = target_user_id
        )
     OR (
          NULLIF(o.payload->>'user_id', '') ~ '^[0-9]+$'
          AND (o.payload->>'user_id')::bigint = target_user_id
        )
     OR o.aggregate_key IN (
          SELECT 'paywall_request:' || p.id::text
          FROM _del_paywall_req_ids p
        )
     OR o.aggregate_key IN (
          SELECT 'refund_request:' || p.id::text
          FROM _del_paywall_req_ids p
        );

  DELETE FROM paywall_access_requests
  WHERE id IN (SELECT id FROM _del_paywall_req_ids)
     OR user_id = target_user_id;

  DELETE FROM training_state WHERE user_id = target_user_id;

  IF to_regclass('public.training_log') IS NOT NULL THEN
    DELETE FROM training_log WHERE user_id = target_user_id;
  END IF;

  SELECT COUNT(*) INTO n_paywall_active
  FROM paywall_access_requests
  WHERE user_id = target_user_id
    AND status = 'completed'
    AND access_expires_at IS NOT NULL
    AND access_expires_at > NOW();

  SELECT COUNT(*) INTO n_training
  FROM training_state
  WHERE user_id = target_user_id;

  SELECT COUNT(*) INTO n_outbox
  FROM outbox_events o
  WHERE o.event_type IN ('paywall_access_restore_requested', 'refund_requested')
    AND (
          (
            NULLIF(o.payload->>'user_id', '') ~ '^[0-9]+$'
            AND (o.payload->>'user_id')::bigint = target_user_id
          )
       OR (
            NULLIF(o.payload->>'UserID', '') ~ '^[0-9]+$'
            AND (o.payload->>'UserID')::bigint = target_user_id
          )
        );

  IF n_paywall_active > 0 OR n_training > 0 OR n_outbox > 0 THEN
    RAISE EXCEPTION
      'Не всё удалено для user_id=%: active_paywall=%, training_state=%, outbox_paywall=%',
      target_user_id, n_paywall_active, n_training, n_outbox;
  END IF;

  RAISE NOTICE 'OK: user_id=% сброшен (paywall/training/outbox=0). /start снова потребует оплату.', target_user_id;
END;
$delete_user$;

COMMIT;

-- Qdrant: session_id = 'personal:<target_user_id>:<pack_chat_id>'
--
-- yookassa_payment_events — НЕ в этой БД (ms_leo / DATABASE_URL бота).
-- Таблица только в опциональной PAYMENT_DATABASE_URL (леджер ms_payments).
-- Если при DELETE получаешь «relation yookassa_payment_events does not exist» —
-- пропусти этот шаг: у тебя леджер отключён, доступ бота живёт в paywall_access_requests.
--
-- Нужно чистить леджер только если в Railway у payment-webhook задан PAYMENT_DATABASE_URL
-- (другая строка подключения, часто БД leo_payments). Подключись к НЕЙ и выполни:
--   DELETE FROM yookassa_payment_events WHERE user_telegram_id = <target_user_id>;
-- DELETE FROM yookassa_payment_events WHERE user_telegram_id =202498592