-- ============================================================================
-- "Честный больничный" для всего чата.
-- Ставит прогресс всех активных участников на паузу и защищает их
-- от удаления из чата (отключает 7-дневный таймер).
--
-- Chat ID: -1003246054143
--
-- Как это работает (см. internal/bot/timer.go):
--   * is_exempt_from_deletion = TRUE  -> бот пропускает юзера при восстановлении
--                                        таймеров и не запускает новый таймер;
--   * has_sick_leave = TRUE, has_healthy = FALSE -> режим больничного (пауза);
--   * timer_start_time = NULL -> юзер вообще не попадает в выборку на удаление
--                                (GetAllUsersWithTimers берёт только NOT NULL).
--
-- ВАЖНО: SQL меняет только БД. Таймеры, которые УЖЕ крутятся в памяти бота
-- (горутины), этим скриптом НЕ отменяются. Чтобы изменения подхватились,
-- нужно перезапустить бота — при старте он перечитает БД и пропустит всех.
-- ============================================================================

BEGIN;

-- Сколько участников затронем (для контроля; ничего не меняет):
-- SELECT COUNT(*) FROM message_log WHERE chat_id = -1003246054143 AND is_deleted = FALSE;

UPDATE message_log
SET
    is_exempt_from_deletion  = TRUE,   -- главное: защита от удаления
    has_sick_leave           = TRUE,   -- режим больничного (пауза прогресса)
    has_healthy              = FALSE,  -- ещё не "выздоровел" -> остаётся на паузе
    timer_start_time         = NULL,   -- сбрасываем отсчёт до удаления
    rest_time_till_del       = NULL,
    sick_approval_pending    = FALSE,  -- снимаем висящие запросы на подтверждение
    sick_approval_deadline   = NULL,
    sick_approval_message_id = NULL,
    updated_at               = NOW()
WHERE chat_id = -1003246054143
  AND is_deleted = FALSE;

-- Проверка результата перед фиксацией:
-- SELECT user_id, username, is_exempt_from_deletion, has_sick_leave, timer_start_time
-- FROM message_log WHERE chat_id = -1003246054143 AND is_deleted = FALSE;

COMMIT;

-- ============================================================================
-- ОТКАТ (вернуть всех в обычный режим — таймеры начнут считаться заново
-- от момента следующего старта таймера):
--
-- UPDATE message_log
-- SET is_exempt_from_deletion = FALSE,
--     has_sick_leave          = FALSE,
--     updated_at              = NOW()
-- WHERE chat_id = -1003246054143
--   AND is_deleted = FALSE;
-- ============================================================================
