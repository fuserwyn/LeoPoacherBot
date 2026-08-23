-- Очистка леджера ЮKassa (таблица yookassa_payment_events).
--
-- Выполняй в БД из PAYMENT_DATABASE_URL (сервис payment-webhook / ms_payments).
-- В основной БД бота (DATABASE_URL ms_leo) этой таблицы НЕТ — будет 42P01.
--
-- Леджер — только аудит вебхуков. Доступ в боте = paywall_access_requests (delete_user_completely.sql).
-- Если PAYMENT_DATABASE_URL не задан в Railway — этот файл не нужен.

BEGIN;

DO $ledger$
DECLARE
  target_user_id bigint := 0;  -- 0 = не удалять по user; >0 = только этот Telegram user_id
  wipe_all boolean := true;    -- true = TRUNCATE всей таблицы; false = только target_user_id (если >0)
  n_before bigint;
  n_after bigint;
BEGIN
  IF to_regclass('public.yookassa_payment_events') IS NULL THEN
    RAISE EXCEPTION
      'Таблица yookassa_payment_events не найдена. Подключись к PAYMENT_DATABASE_URL (leo_payments), не к БД бота.';
  END IF;

  SELECT COUNT(*) INTO n_before FROM yookassa_payment_events;

  IF wipe_all THEN
    TRUNCATE TABLE yookassa_payment_events RESTART IDENTITY;
    RAISE NOTICE 'TRUNCATE: удалены все % строк леджера yookassa_payment_events', n_before;
  ELSIF target_user_id IS NULL OR target_user_id <= 0 THEN
    RAISE EXCEPTION 'wipe_all=false: укажи target_user_id > 0';
  ELSE
    DELETE FROM yookassa_payment_events WHERE user_telegram_id = target_user_id;
    GET DIAGNOSTICS n_after = ROW_COUNT;
    RAISE NOTICE 'DELETE: user_id=% удалено строк: % (было всего в таблице: %)',
      target_user_id, n_after, n_before;
  END IF;

  SELECT COUNT(*) INTO n_after FROM yookassa_payment_events;
  RAISE NOTICE 'Осталось строк в yookassa_payment_events: %', n_after;
END;
$ledger$;

COMMIT;
