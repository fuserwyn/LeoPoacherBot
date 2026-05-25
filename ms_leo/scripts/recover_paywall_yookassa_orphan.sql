-- Восстановление после «деньги списались, вебхук 403 chat mismatch».
-- payment_id из лога ЮKassa, req из invoice_payload pw_<id>.
--
-- Вариант A — выдать доступ (если оплата должна засчитаться):
--   1) Выровнять pack id в заявке с MONETIZED_CHAT_ID ms_leo/ms_payments
--   2) Пользователь пишет /start в бот (paywallTrySyncYookassaPayment) ИЛи повтор вебхука после деплоя
--
-- Вариант B — возврат через outbox (как при dead restore):
--   INSERT refund_requested — обработает ms_leo outbox_worker (нужны YOOKASSA_* в ms_leo).

\set req_id 8
\set user_id 0   -- подставь user_telegram_id из metadata вебхука
\set env_chat -1003743863056

-- Диагностика
SELECT id, user_id, monetized_chat_id, status, yookassa_payment_id, created_at, completed_at
FROM paywall_access_requests
WHERE id = :req_id;

-- A: выровнять pack id (после деплоя вебхук больше не отдаёт 403 на chat)
-- UPDATE paywall_access_requests
-- SET monetized_chat_id = :env_chat
-- WHERE id = :req_id AND status = 'pending';

-- B: поставить возврат в очередь (замени :user_id)
/*
INSERT INTO outbox_events (event_type, aggregate_key, payload, status, next_attempt_at)
VALUES (
  'refund_requested',
  'refund_request:' || :req_id::text,
  jsonb_build_object(
    'request_id', :req_id,
    'user_id', :user_id,
    'reason', 'manual: yookassa webhook chat mismatch before fix'
  ),
  'pending',
  NOW()
);
*/

-- Проверка outbox
-- SELECT id, event_type, aggregate_key, status, last_error, created_at
-- FROM outbox_events
-- WHERE aggregate_key = 'refund_request:' || :req_id::text
-- ORDER BY id DESC;
