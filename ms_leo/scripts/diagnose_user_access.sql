-- Диагностика: почему у пользователя есть доступ после delete_user_completely.sql
-- Подставь telegram user_id и MONETIZED_CHAT_ID (как в Railway env у ms_leo) в двух местах ниже.

DO $diag$
DECLARE
  target_user_id bigint := 123456789;
  monetized_chat_id bigint := -1001234567890;
  n_active_paywall bigint;
  n_training bigint;
  n_outbox_pending bigint;
BEGIN
  RAISE NOTICE '=== paywall_access_requests (последние 10) ===';
  FOR r IN
    SELECT id, status, monetized_chat_id, created_at, completed_at, access_expires_at,
           yookassa_payment_id, telegram_payment_charge_id
    FROM paywall_access_requests
    WHERE user_id = target_user_id
    ORDER BY id DESC
    LIMIT 10
  LOOP
    RAISE NOTICE 'id=% status=% chat=% created=% completed=% expires=% yk=% tg_charge=%',
      r.id, r.status, r.monetized_chat_id, r.created_at, r.completed_at, r.access_expires_at,
      r.yookassa_payment_id, r.telegram_payment_charge_id;
  END LOOP;

  SELECT COUNT(*) INTO n_active_paywall
  FROM paywall_access_requests
  WHERE user_id = target_user_id
    AND monetized_chat_id = monetized_chat_id
    AND status = 'completed'
    AND access_expires_at IS NOT NULL
    AND access_expires_at > NOW();

  SELECT COUNT(*) INTO n_training
  FROM training_state
  WHERE user_id = target_user_id;

  SELECT COUNT(*) INTO n_outbox_pending
  FROM outbox_events
  WHERE event_type = 'paywall_access_restore_requested'
    AND status IN ('pending', 'processing', 'retry')
    AND (
      (NULLIF(payload->>'user_id', '') ~ '^[0-9]+$' AND (payload->>'user_id')::bigint = target_user_id)
      OR (NULLIF(payload->>'UserID', '') ~ '^[0-9]+$' AND (payload->>'UserID')::bigint = target_user_id)
    );

  RAISE NOTICE 'has_active_paywall (бот считает оплаченным) = %', n_active_paywall;
  RAISE NOTICE 'training_state rows = %', n_training;
  RAISE NOTICE 'pending paywall outbox events = %', n_outbox_pending;

  IF n_active_paywall > 0 THEN
    RAISE NOTICE 'Причина доступа: в БД есть completed paywall. Повтори delete_user_completely.sql на ЭТОЙ БД.';
  ELSIF n_outbox_pending > 0 THEN
    RAISE NOTICE 'Причина: висит outbox — дождись обработки или удали outbox и перезапусти delete.';
  ELSE
    RAISE NOTICE 'В БД активной оплаты нет. Если бот всё равно даёт доступ — см. OWNER_ID/ADMIN_IDS, PAYWALL_ENABLED=false, или деплой без фикса paywall.';
  END IF;
END;
$diag$;
